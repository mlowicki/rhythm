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
	Logging     Logging
}

type API struct {
	Address  string
	CertFile string
	KeyFile  string
	Auth     APIAuth
}

const (
	APIAuthBackendGitLab = "gitlab"
	APIAuthBackendNone   = "none"
)

type APIAuth struct {
	Backend string
	GitLab  APIAuthGitLab
}

type APIAuthGitLab struct {
	BaseURL string
}

type Storage struct {
	Backend   string
	ZooKeeper StorageZK
}

const StorageBackendZK = "zookeeper"

type StorageZK struct {
	Dir     string
	Servers []string
	Timeout time.Duration
	Auth    ZKAuth
}

const CoordinatorBackendZK = "zookeeper"

type Coordinator struct {
	Backend   string
	ZooKeeper CoordinatorZK
}

type CoordinatorZK struct {
	Dir     string
	Servers []string
	Timeout time.Duration
	Auth    ZKAuth
}

const (
	ZKAuthSchemeDigest = "digest"
	ZKAuthSchemeWorld  = "world"
)

type ZKAuth struct {
	Scheme string
	Digest ZKAuthDigest
}

type ZKAuthDigest struct {
	User     string
	Password string
}

const SecretsBackendVault = "vault"

type Secrets struct {
	Backend string
	Vault   SecretsVault
}

type SecretsVault struct {
	Token   string
	Address string
	Timeout time.Duration
	Root    string
}

type Mesos struct {
	Auth            MesosAuth
	BaseURL         string
	RootCA          string
	Checkpoint      bool
	FailoverTimeout time.Duration
	Hostname        string
	User            string
	WebUiURL        string
	Principal       string
	Labels          map[string]string
	Roles           []string
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

const (
	LoggingBackendNone   = "none"
	LoggingBackendSentry = "sentry"
)

type Logging struct {
	Backend string
	Sentry  LoggingSentry
}

type LoggingSentry struct {
	DSN    string
	RootCA string
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
				Backend: APIAuthBackendNone,
			},
		},
		Storage: Storage{
			Backend: StorageBackendZK,
			ZooKeeper: StorageZK{
				Servers: []string{"127.0.0.1"},
				Timeout: 10000, // 10s
				Dir:     "rhythm",
				Auth: ZKAuth{
					Scheme: ZKAuthSchemeWorld,
				},
			},
		},
		Coordinator: Coordinator{
			Backend: CoordinatorBackendZK,
			ZooKeeper: CoordinatorZK{
				Servers: []string{"127.0.0.1"},
				Timeout: 10000, // 10s
				Dir:     "rhythm",
				Auth: ZKAuth{
					Scheme: ZKAuthSchemeWorld,
				},
			},
		},
		Secrets: Secrets{
			Backend: SecretsBackendVault,
			Vault: SecretsVault{
				Timeout: 3000, // 3s
				Root:    "secret/rhythm/",
			},
		},
		Verbose: false,
		Mesos: Mesos{
			BaseURL:         "http://127.0.0.1:5050",
			FailoverTimeout: time.Hour * 24 * 7,
			Roles:           []string{"*"},
			Auth: MesosAuth{
				Type: MesosAuthTypeBasic,
			},
		},
		Logging: Logging{
			Backend: LoggingBackendNone,
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
