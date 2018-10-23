package conf

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
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
	TaskTTL time.Duration
}

const CoordinatorBackendZK = "zookeeper"

type Coordinator struct {
	Backend   string
	ZooKeeper CoordinatorZK
}

type CoordinatorZK struct {
	Dir         string
	Addrs       []string
	Timeout     time.Duration
	Auth        ZKAuth
	ElectionDir string
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

func millisecondFieldsToDuration(v reflect.Value) {
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Struct {
			millisecondFieldsToDuration(v.Field(i))
		} else {
			if v.Field(i).Type() == reflect.TypeOf(time.Second) {
				if v.Field(i).CanSet() {
					d := v.Field(i).Interface().(time.Duration)
					d *= time.Millisecond
					v.Field(i).Set(reflect.ValueOf(d))
				}
			}
		}
	}
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
				TaskTTL: 1000 * 3600 * 24, // 24h
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
			FailoverTimeout: 1000 * 3600 * 24 * 7, // 7d
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
	conf.Coordinator.ZooKeeper.ElectionDir = "election/mesos_scheduler"
	// All time.Duration fields from Conf should be in milliseconds so
	// conversion to time elapsed in nanoseconds (represented by time.Duration)
	// is needed.
	millisecondFieldsToDuration(reflect.ValueOf(conf).Elem())
	return conf, nil
}
