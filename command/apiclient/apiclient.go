package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mlowicki/rhythm/model"
	log "github.com/sirupsen/logrus"
)

const envRhythmAddr = "RHYTHM_ADDR"

// HealthInfo describes server status.
type HealthInfo struct {
	Leader     bool
	Version    string
	ServerTime string
}

// New creates instance of API client.
func New(addr string, auth func(*http.Request) error) (*Client, error) {
	c := Client{
		auth: auth,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
	if v := os.Getenv(envRhythmAddr); addr == "" && v != "" {
		addr = v
	}
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing server address: %s.", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("Invalid server address scheme: %s.", u.Scheme)
	}
	if u.Scheme == "http" {
		log.Warnf("HTTP scheme is used to talk with Rhythm API. HTTPS is highly recommended.")
	}
	c.addr = u
	return &c, nil
}

// Client describes API client.
type Client struct {
	addr       *url.URL
	auth       func(*http.Request) error
	httpClient *http.Client
}

func (c *Client) parseErrResp(body []byte) error {
	var errs *multierror.Error
	var resp struct {
		Errors []string
	}
	err := json.Unmarshal(body, &resp)
	if err != nil {
		return fmt.Errorf("Error decoding response: %s (%s)", err, body)
	}
	for _, err := range resp.Errors {
		errs = multierror.Append(errs, fmt.Errorf(err))
	}
	return errs
}

func (c *Client) send(req *http.Request, auth func(*http.Request) error) (*http.Response, error) {
	if auth != nil {
		err := auth(req)
		if err != nil {
			return nil, fmt.Errorf("Authentication failed: %s.", err)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error sending request: %s.", err)
	}
	return resp, nil
}

// Health returns server status info.
func (c *Client) Health() (*HealthInfo, error) {
	u, _ := url.Parse(c.addr.String())
	u.Path = "api/v1/health"
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Server error: %d.", resp.StatusCode)
	}
	var health HealthInfo
	err = json.Unmarshal(body, &health)
	if err != nil {
		return nil, fmt.Errorf("Error decoding server status: %s.", err)
	}
	return &health, nil
}

// ReadTasks returns list of job's runs.
func (c *Client) ReadTasks(fqid string) ([]*model.Task, error) {
	u, _ := url.Parse(c.addr.String())
	u.Path = fmt.Sprintf("api/v1/jobs/%s/tasks", fqid)
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, c.auth)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Job not found.")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrResp(body)
	}
	var tasks []*model.Task
	err = json.Unmarshal(body, &tasks)
	if err != nil {
		return nil, fmt.Errorf("Error decoding tasks: %s.", err)
	}
	return tasks, nil
}

// ReadJob returns job's info.
func (c *Client) ReadJob(fqid string) (*model.Job, error) {
	if strings.Count(fqid, "/") != 2 {
		return nil, fmt.Errorf("Invalid job ID.")
	}
	u, _ := url.Parse(c.addr.String())
	u.Path = "api/v1/jobs/" + fqid
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, c.auth)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Job not found.")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrResp(body)
	}
	var job model.Job
	err = json.Unmarshal(body, &job)
	if err != nil {
		return nil, fmt.Errorf("Error decoding job: %s.", err)
	}
	return &job, nil
}

// DeleteJob removes job.
func (c *Client) DeleteJob(fqid string) error {
	u, _ := url.Parse(c.addr.String())
	u.Path = "api/v1/jobs/" + fqid
	req, err := http.NewRequest("DELETE", u.String(), nil)
	if err != nil {
		return fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, c.auth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

// RunJob schedules job for immeddiate run.
func (c *Client) RunJob(fqid string) error {
	u, _ := url.Parse(c.addr.String())
	u.Path = "api/v1/jobs/" + fqid + "/run"
	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, c.auth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

// CreateJob adds new job.
func (c *Client) CreateJob(jobEncoded []byte) error {
	u, _ := url.Parse(c.addr.String())
	u.Path = "api/v1/jobs"
	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(jobEncoded))
	if err != nil {
		return fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, c.auth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

// UpdateJob modifies existing job.
func (c *Client) UpdateJob(fqid string, changesEncoded []byte) error {
	u, _ := url.Parse(c.addr.String())
	u.Path = "api/v1/jobs/" + fqid
	req, err := http.NewRequest("PUT", u.String(), bytes.NewReader(changesEncoded))
	if err != nil {
		return fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, c.auth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

// FindJobs returns jobs matching filter.
func (c *Client) FindJobs(filter string) ([]*model.Job, error) {
	if strings.Count(filter, "/") > 1 {
		return nil, fmt.Errorf("Invalid filter.")
	}
	u, _ := url.Parse(c.addr.String())
	u.Path = fmt.Sprintf("api/v1/jobs/%s", filter)
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s.", err)
	}
	resp, err := c.send(req, c.auth)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s.", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrResp(body)
	}
	var jobs []*model.Job
	err = json.Unmarshal(body, &jobs)
	if err != nil {
		return nil, fmt.Errorf("Error decoding jobs: %s.", err)
	}
	return jobs, nil
}
