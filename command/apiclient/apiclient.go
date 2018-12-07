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

type HealthInfo struct {
	Leader     bool
	Version    string
	ServerTime string
}

func New(addr string, authReq func(*http.Request) error) *Client {
	c := Client{
		addr:    addr,
		authReq: authReq,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
	return &c
}

type Client struct {
	addr       string
	authReq    func(*http.Request) error
	httpClient *http.Client
}

func (c *Client) getAddr() (*url.URL, error) {
	var addr string
	if v := os.Getenv(envRhythmAddr); v != "" {
		addr = v
	}
	if c.addr != "" {
		addr = c.addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing server address: %s", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("Invalid server address scheme: %s", u.Scheme)
	}
	if u.Scheme == "http" {
		log.Warnf("HTTP scheme is used to talk with Rhythm API. Consider using HTTPS")
	}
	return u, nil
}

func (c *Client) parseErrResp(body []byte) error {
	var errs *multierror.Error
	var resp struct {
		Errors []string
	}
	err := json.Unmarshal(body, &resp)
	if err != nil {
		return fmt.Errorf("Error decoding response: %s", err)
	}
	for _, err := range resp.Errors {
		errs = multierror.Append(errs, fmt.Errorf(err))
	}
	return errs
}

func (c *Client) send(req *http.Request) (*http.Response, error) {
	if c.authReq != nil {
		err := c.authReq(req)
		if err != nil {
			return nil, fmt.Errorf("Authentication failed: %s", err)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error reading tasks: %s", err)
	}
	return resp, nil
}

func (c *Client) Health() (*HealthInfo, error) {
	u, err := c.getAddr()
	if err != nil {
		return nil, err
	}
	u.Path = "api/v1/health"
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Server error: %d", resp.StatusCode)
	}
	var health HealthInfo
	err = json.Unmarshal(body, &health)
	if err != nil {
		return nil, fmt.Errorf("Error decoding server status: %s", err)
	}
	return &health, nil
}

func (c *Client) ReadTasks(fqid string) ([]*model.Task, error) {
	u, err := c.getAddr()
	if err != nil {
		return nil, err
	}
	u.Path = fmt.Sprintf("api/v1/jobs/%s/tasks", fqid)
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrResp(body)
	}
	var tasks []*model.Task
	err = json.Unmarshal(body, &tasks)
	if err != nil {
		return nil, fmt.Errorf("Error decoding tasks: %s", err)
	}
	return tasks, nil
}

func (c *Client) ReadJob(fqid string) (*model.Job, error) {
	if strings.Count(fqid, "/") != 2 {
		return nil, fmt.Errorf("Invalid job ID")
	}
	u, err := c.getAddr()
	if err != nil {
		return nil, err
	}
	u.Path = "api/v1/jobs/" + fqid
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Job not found")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrResp(body)
	}
	var job model.Job
	err = json.Unmarshal(body, &job)
	if err != nil {
		return nil, fmt.Errorf("Error decoding job: %s", err)
	}
	return &job, nil
}

func (c *Client) DeleteJob(fqid string) error {
	u, err := c.getAddr()
	if err != nil {
		return err
	}
	u.Path = "api/v1/jobs/" + fqid
	req, err := http.NewRequest("DELETE", u.String(), nil)
	if err != nil {
		return fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

func (c *Client) RunJob(fqid string) error {
	u, err := c.getAddr()
	if err != nil {
		return err
	}
	u.Path = "api/v1/jobs/" + fqid + "/run"
	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

func (c *Client) CreateJob(jobEncoded []byte) error {
	u, err := c.getAddr()
	if err != nil {
		return err
	}
	u.Path = "api/v1/jobs"
	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(jobEncoded))
	if err != nil {
		return fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

func (c *Client) UpdateJob(fqid string, changesEncoded []byte) error {
	u, err := c.getAddr()
	if err != nil {
		return err
	}
	u.Path = "api/v1/jobs/" + fqid
	req, err := http.NewRequest("PUT", u.String(), bytes.NewReader(changesEncoded))
	if err != nil {
		return fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return c.parseErrResp(body)
	}
	return nil
}

func (c *Client) FindJobs(filter string) ([]*model.Job, error) {
	if strings.Count(filter, "/") > 1 {
		return nil, fmt.Errorf("Invalid filter")
	}
	u, err := c.getAddr()
	if err != nil {
		return nil, err
	}
	u.Path = fmt.Sprintf("api/v1/jobs/%s", filter)
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s", err)
	}
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrResp(body)
	}
	var jobs []*model.Job
	err = json.Unmarshal(body, &jobs)
	if err != nil {
		return nil, fmt.Errorf("Error decoding jobs: %s", err)
	}
	return jobs, nil
}
