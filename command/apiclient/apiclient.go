package apiclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/mlowicki/rhythm/model"
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
	}
	return &c
}

type Client struct {
	addr    string
	authReq func(*http.Request) error
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
	return u, nil
}

func (c *Client) Health() (*HealthInfo, error) {
	u, err := c.getAddr()
	if err != nil {
		return nil, err
	}
	u.Path = "api/v1/health"
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("Error getting server status: %s", err)
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

func (c *Client) GetJob(fqid string) (*model.Job, error) {
	u, err := c.getAddr()
	if err != nil {
		return nil, err
	}
	u.Path = "api/v1/jobs/" + fqid
	req, err := http.NewRequest("GET", u.String(), nil)
	err = c.authReq(req)
	if err != nil {
		return nil, fmt.Errorf("Authentication failed: %s", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error getting job: %s", err)
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
