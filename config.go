package main

import (
	"encoding/json"
	"io/ioutil"
	"time"
)

type Config struct {
	GitLab    ConfigGitLab
	API       ConfigAPI
	Vault     ConfigVault
	ZooKeeper ConfigZooKeeper
	Verbose   bool
	Mesos     ConfigMesos
}

type ConfigAPI struct {
	Address string
}

type ConfigVault struct {
	Token   string
	Address string
	Timeout time.Duration
}

type ConfigGitLab struct {
	BaseURL string
}

type ConfigZooKeeper struct {
	BasePath    string
	ElectionDir string
	Servers     []string
	Timeout     time.Duration
}

type ConfigMesos struct {
	Auth            ConfigMesosAuth
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

type ConfigMesosAuth struct {
	Type  string
	Basic ConfigMesosAuthBasic
}

type ConfigMesosAuthBasic struct {
	Username string
	Password string
}

func getConfig(path string) (*Config, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conf = &Config{
		API: ConfigAPI{
			Address: "localhost:8000",
		},
		Vault: ConfigVault{
			Timeout: 3000, // 3s
		},
		Verbose: false,
		Mesos: ConfigMesos{
			BaseURL:         "http://127.0.0.1:5050",
			FailoverTimeout: time.Hour * 24 * 7,
		},
		ZooKeeper: ConfigZooKeeper{
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
