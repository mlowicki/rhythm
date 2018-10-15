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
	Mesos       Mesos
	Logging     Logging
}

type API struct {
	Addr     string
	CertFile string
	KeyFile  string
	Auth     APIAuth
}

const (
	APIAuthBackendGitLab = "gitlab"
	APIAuthBackendNone   = "none"
	APIAuthBackendLDAP   = "ldap"
)

type APIAuth struct {
	Backend string
	GitLab  APIAuthGitLab
	LDAP    APIAuthLDAP
}

type APIAuthGitLab struct {
	Addr   string
	CACert string
}

type APIAuthLDAP struct {
	Addrs              []string
	UserDN             string
	UserAttr           string
	CACert             string
	UserACL            map[string]map[string]string
	GroupACL           map[string]map[string]string
	BindDN             string
	BindPassword       string
	Timeout            time.Duration
	GroupFilter        string
	GroupDN            string
	GroupAttr          string
	CaseSensitiveNames bool
}

type Storage struct {
	Backend   string
	ZooKeeper StorageZK
}

const StorageBackendZK = "zookeeper"

type StorageZK struct {
	Dir     string
	Addrs   []string
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
	Addrs   []string
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

const (
	SecretsBackendVault = "vault"
	SecretsBackendNone  = "none"
)

type Secrets struct {
	Backend string
	Vault   SecretsVault
}

type SecretsVault struct {
	Token   string
	Addr    string
	Timeout time.Duration
	Root    string
	CACert  string
}

type Mesos struct {
	Auth            MesosAuth
	Addrs           []string
	CACert          string
	Checkpoint      bool
	FailoverTimeout time.Duration
	Hostname        string
	User            string
	WebUiURL        string
	Principal       string
	Labels          map[string]string
	Roles           []string
	LogAllEvents    bool
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
	LoggingLevelDebug    = "debug"
	LoggingLevelInfo     = "info"
	LoggingLevelWarn     = "warn"
	LoggingLevelError    = "error"
)

type Logging struct {
	Level   string
	Backend string
	Sentry  LoggingSentry
}

type LoggingSentry struct {
	DSN    string
	CACert string
	Tags   map[string]string
}

func New(path string) (*Conf, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conf = &Conf{
		API: API{
			Addr: "localhost:8000",
			Auth: APIAuth{
				Backend: APIAuthBackendNone,
				LDAP: APIAuthLDAP{
					Timeout:     5000,
					GroupFilter: "(|(memberUid={{.Username}})(member={{.UserDN}})(uniqueMember={{.UserDN}}))",
					GroupAttr:   "cn",
				},
			},
		},
		Storage: Storage{
			Backend: StorageBackendZK,
			ZooKeeper: StorageZK{
				Addrs:   []string{"127.0.0.1"},
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
				Addrs:   []string{"127.0.0.1"},
				Timeout: 10000, // 10s
				Dir:     "rhythm",
				Auth: ZKAuth{
					Scheme: ZKAuthSchemeWorld,
				},
			},
		},
		Secrets: Secrets{
			Backend: SecretsBackendNone,
			Vault: SecretsVault{
				Timeout: 0, // no timeout
				Root:    "secret/rhythm/",
			},
		},
		Mesos: Mesos{
			FailoverTimeout: 1000 * 3600 * 24 * 7, // 7 days
			Roles:           []string{"*"},
			Auth: MesosAuth{
				Type: MesosAuthTypeNone,
			},
		},
		Logging: Logging{
			Backend: LoggingBackendNone,
			Level:   LoggingLevelInfo,
		},
	}
	err = json.Unmarshal(file, conf)
	if err != nil {
		return nil, err
	}
	conf.Mesos.FailoverTimeout *= time.Millisecond
	conf.Secrets.Vault.Timeout *= time.Millisecond
	conf.Storage.ZooKeeper.Timeout *= time.Millisecond
	conf.Coordinator.ZooKeeper.Timeout *= time.Millisecond
	conf.API.Auth.LDAP.Timeout *= time.Millisecond
	return conf, nil
}
