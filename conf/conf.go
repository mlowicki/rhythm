package conf

import (
	"encoding/json"
	"io/ioutil"
	"time"
)

type Conf struct {
	API         API
	Storage     Storage
	Coordinator Coordinator
	Secrets     Secrets
	Verbose     bool
	Mesos       Mesos
}

type API struct {
	Address string
	Auth    APIAuth
}

const (
	APIAuthTypeGitLab = "gitlab"
	APIAuthTypeNone   = "none"
)

type APIAuth struct {
	Type   string
	GitLab GitLab
}

type GitLab struct {
	BaseURL string
}

type Storage struct {
	Type      string
	ZooKeeper StorageZooKeeper
}

const StorageTypeZooKeeper = "zookeeper"

type StorageZooKeeper struct {
	BasePath string
	Servers  []string
	Timeout  time.Duration
}

const CoordinatorTypeZooKeeper = "zookeeper"

type Coordinator struct {
	Type      string
	ZooKeeper CoordinatorZooKeeper
}

type CoordinatorZooKeeper struct {
	BasePath    string
	ElectionDir string
	Servers     []string
	Timeout     time.Duration
}

const SecretsTypeVault = "vault"

type Secrets struct {
	Type  string
	Vault SecretsVault
}

type SecretsVault struct {
	Token   string
	Address string
	Timeout time.Duration
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
	MesosAuthTypeBasic = "basic"
	MesosAuthTypeNone  = "none"
)

type MesosAuth struct {
	Type  string
	Basic MesosAuthBasic
}

type MesosAuthBasic struct {
	Username string
	Password string
}

func New(path string) (*Conf, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conf = &Conf{
		API: API{
			Address: "localhost:8000",
			Auth: APIAuth{
				Type: APIAuthTypeNone,
			},
		},
		Storage: Storage{
			Type: "zookeeper",
			ZooKeeper: StorageZooKeeper{
				Servers:  []string{"127.0.0.1"},
				Timeout:  10000, // 10s
				BasePath: "/rhythm",
			},
		},
		Coordinator: Coordinator{
			Type: "zookeeper",
			ZooKeeper: CoordinatorZooKeeper{
				Servers:     []string{"127.0.0.1"},
				Timeout:     10000, // 10s
				BasePath:    "/rhythm",
				ElectionDir: "election",
			},
		},
		Secrets: Secrets{
			Type: "vault",
			Vault: SecretsVault{
				Timeout: 3000, // 3s
			},
		},
		Verbose: false,
		Mesos: Mesos{
			BaseURL:         "http://127.0.0.1:5050",
			FailoverTimeout: time.Hour * 24 * 7,
		},
	}
	err = json.Unmarshal(file, conf)
	conf.Secrets.Vault.Timeout *= time.Millisecond
	conf.Storage.ZooKeeper.Timeout *= time.Millisecond
	conf.Coordinator.ZooKeeper.Timeout *= time.Millisecond
	if err != nil {
		return nil, err
	}
	return conf, nil
}
