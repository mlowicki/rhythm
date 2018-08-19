package conf

import (
	"encoding/json"
	"io/ioutil"
	"time"
)

type Conf struct {
	GitLab    GitLab
	API       API
	Vault     Vault
	ZooKeeper ZooKeeper
	Verbose   bool
	Mesos     Mesos
}

type API struct {
	Address string
}

type Vault struct {
	Token   string
	Address string
	Timeout time.Duration
}

type GitLab struct {
	BaseURL string
}

type ZooKeeper struct {
	BasePath    string
	ElectionDir string
	Servers     []string
	Timeout     time.Duration
}

type Mesos struct {
	Auth            MesosAuth
	BaseURL         string
	Checkpoint      bool
	FailoverTimeout time.Duration
	Hostname        string
	User            string
	WebUiURL        string
	Principal       string
}

const (
	AuthModeBasic = "basic"
	AuthModeNone  = "none"
)

type MesosAuth struct {
	Type  string
	Basic MesosAuthBasic
}

type MesosAuthBasic struct {
	Username string
	Password string
}

func NewConf(path string) (*Conf, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conf = &Conf{
		API: API{
			Address: "localhost:8000",
		},
		Vault: Vault{
			Timeout: 3000, // 3s
		},
		Verbose: false,
		Mesos: Mesos{
			BaseURL:         "http://127.0.0.1:5050",
			FailoverTimeout: time.Hour * 24 * 7,
		},
		ZooKeeper: ZooKeeper{
			Servers:     []string{"127.0.0.1"},
			Timeout:     10000, // 10s
			BasePath:    "/rhythm",
			ElectionDir: "election",
		},
	}
	err = json.Unmarshal(file, conf)
	conf.Vault.Timeout *= time.Millisecond
	conf.ZooKeeper.Timeout *= time.Millisecond
	if err != nil {
		return nil, err
	}
	return conf, nil
}
