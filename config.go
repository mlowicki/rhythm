package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"
)

type Config struct {
	GitLab          ConfigGitLab
	API             ConfigAPI
	Vault           ConfigVault
	Storage         string
	ZooKeeper       ConfigZooKeeper
	FailoverTimeout float64
	Verbose         bool
	Mesos           ConfigMesos
}

type ConfigAPI struct {
	Address string
}

type ConfigVault struct {
	Token   string
	Address string
	Timeout int
}

type ConfigGitLab struct {
	BaseURL string
}

type ConfigZooKeeper struct {
	BasePath string
}

type ConfigMesos struct {
	BaseURL string
	Auth    ConfigMesosAuth
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
			Timeout: 3,
		},
		FailoverTimeout: (time.Hour * 24 * 7).Seconds(),
		Verbose:         false,
		Mesos: ConfigMesos{
			BaseURL: "http://127.0.0.1:5050",
		},
	}
	err = json.Unmarshal(file, conf)
	if err != nil {
		return nil, err
	}
	checkConfig(conf)
	return conf, nil
}

func checkConfig(conf *Config) {
	_, err := newGitLabClient(conf, "")
	if err != nil {
		log.Fatal(err)
	}
}
