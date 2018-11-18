# Rhythm

## Features

* Support for [Docker](https://mesos.apache.org/documentation/latest/docker-containerizer/) and [Mesos](https://mesos.apache.org/documentation/latest/mesos-containerizer/) Containerizers 
* Integration with [HashiCorp Vault](https://www.vaultproject.io/) for secrets management
* Access control list (ACL) backed by [GitLab](https://gitlab.com/) or LDAP
* [Cron syntax](http://www.nncron.ru/help/EN/working/cron-format.htm)
* Integration with [Sentry](https://sentry.io/) for error tracking
* Command-line client ([Documentation](#command-line-client))

## Glossary

* job - periodic action to perform (e.g. database backup).
* task - single run (instance) of job. If task fails and job allows for retries then new task will be launched.

## API

[Documentation](https://mlowicki.github.io/rhythm/api)

## Configuration

Rhythm server can be started with:
```
rhythm server -config=/etc/rhythm/config.json
```

Server is configured using file in JSON format. By default config.json from current  directory is used but it can overwritten using `-config` parameter.
There are couple of sections in configuration file:
* api
* storage
* coordinator
* secrets
* mesos
* logging

### API

Options:
* addr (optional) - Address (without scheme) API server will bind to. If `certfile` or `keyfile` is set then HTTPS will be used, otherwise HTTP (`"localhost:8000"` by default).
* certfile (optional) - Absolute path to certificate.
* keyfile (optional) - Absolute path to private key.
* auth (optional)
	* backend (optional) - `"none"`, `"gitlab"` or `"ldap"` (`"none"` by default).
	* gitlab (optional and used only if `backend` is set to `"gitlab"`)
		* addr (required) - GitLab address with scheme like `https://`.
		* cacert (optional) - Absolute path to CA certificate to use when verifying GitLab server certificate, must be x509 PEM encoded.
	* ldap (optional and used only if `backend` is set to `"ldap"`)
		* addrs (required) - List of LDAP server addresses with scheme (`"ldap://"` or `"ldaps://"`) and optional port.
		* userdn (required) - Base DN under which to perform user search.
		* userattr (required) - Attribute on user object matching the username passed when authenticating (`"cn"` by default).
		* binddn (optional) - Distinguished name of object to bind when performing user search.
		* bindpassword (optional) - Password to use along with `binddn` when performing user search.
		* cacert (optional) - Absolute path to CA certificate to use when verifying LDAP server certificate, must be x509 PEM encoded.
		* timeout (optional) - Connect timeout in milliseconds for single connection to one server (`5000` by default).
		* caseSensitiveNames (optional) - If set, LDAP user and group names lookups in `useracl` and `groupacl` will be case sensitive. Otherwise, names will be normalized to lower case - use lowe case in `groupacl` and `useracl` to make matching work. Case will still be preserved when sending the username to the LDAP server at login time; this is only for matching user/group ACLs. (`false` by default).
		* groupfilter (optional) - Go template used when constructing the group membership query. The template can access the following context variables: [`UserDN`, `Username`]. Use `"(&(objectClass=group)(member:1.2.840.113556.1.4.1941:={{.UserDN}}))"` to support nested group resolution for Active Directory (`"(|(memberUid={{.Username}})(member={{.Us    erDN}})(uniqueMember={{.UserDN}}))"` by default).
		* groupdn (required) - LDAP search base to use for group membership search like `"ou=Groups,dc=example,dc=com"`. This can be the root containing either groups or users.
		* groupattr (required) - LDAP attribute to follow on objects returned by `groupfilter` in order to enumerate user group membership. Examples: for groupfilter queries returning group objects, use: `"cn"`. For queries returning user objects, use: `"memberOf"` (`"cn"` by default).
		* useracl (optional) - Access control list defining permission level (`"readwrite"` or `"readonly"`) per LDAP user. Possible to define it for project, the whole group or everything / all projects (`"*"`).
		Example:
		```javascript
		"useracl": {
			"john": {
				"infra": "readonly",
				"infra/monitoring": "readwrite"
			},
			"katie":  {
				"infra/monitoring": "readwrite",
				"*": "readonly"
			}
		}
		```
		If accesses for both the whole group and for specific project in that group are set then latter is used (`john` has read-write access to `infra/monitoring` and read-only access to any other project from `infra` group).

		Match-all (`"*"`) has the least priority so if set and also group or project is specified then latter is used (`katie` has read-write access to `infra/monitoring` and read-only access to any other project).

		See [API documentation](https://mlowicki.github.io/rhythm/api#header-ldap) for explanation which access level is taking into account if both `useracl` and `groupacl` are set.
		* groupacl (optional) - Access control list defining permission level (`"readwrite"` or `"readonly"`) per LDAP group. Possible to define it for project, the whole group or everything / all projects (`"*"`).
		Example:
		```javascript
		"groupacl": {
			"qa": {
				"infra": "readonly",
				"infra/monitoring": "readwrite"
			},
			"devs":  {
				"infra/monitoring": "readwrite",
				"*": "readonly"
			}
		}
		```
		If accesses for both the whole group and for specific project in that group are set then latter is used (`qa` LDAP group members have read-write access to `infra/monitoring` and read-only access to any other project from `infra` group).

		Match-all (`"*"`) has the least priority so if set and also group or project is specified then latter is used (`devs` LDAP group members have read-write access to `infra/monitoring` and read-only access to any other project).

		See [API documentation](https://mlowicki.github.io/rhythm/api#header-ldap) for explanation which access level is taking into account if both `useracl` and `groupacl` are set.

Examples:
```javascript
"api": {
    "addr": "localhost:8888",
    "auth": {
        "backend": "gitlab",
        "gitlab": {
            "addr": "https://example.com",
            "cacert": "/var/ca.crt"
        }
    }
}
```

```javascript
"api": {
    "addr": "localhost:8888",
    "auth": {
        "backend": "ldap",
        "ldap": {
            "addrs": ["ldap://example.com"],
            "userdn": "dc=example,dc=com",
            "userattr": "uid"
        }
    }
}
```

### Storage

Options:
* backend (optional) - `"zookeeper"` (`"zookeeper"` by default).
* zookeeper (optional and used only if `backend` is set to `"zookeeper"`)
	* dir - Location (name without slashes) to store data (`"rhythm"` by default).
	* addrs - Servers locations without scheme. If port is not set then default `2181` will be used (`["127.0.0.1"]` by default).
	* timeout (optional) - ZooKeeper client timeout in milliseconds (10s by default).
	* auth (optional)
		* scheme (optional) - `"digest"` or `"world"` (`"world"` by default).
		* digest (optional and used only if `scheme` is set to `"digest"`)
			* user (optional)
			* password (optional)
    * taskttl (optional) - number of milliseconds record of runned task should be kept (`7 days` by default).

Example:
```javascript
"storage": {
    "zookeeper": {
        "addrs": ["192.168.0.1", "192.168.0.2", "192.168.0.3"],
        "dir": "rhythm",
        "timeout": 20000,
        "auth": {
            "scheme": "digest",
            "digest": {
                "user": "john",
                "password": "secret"
            }
        }
    }
}
```

### Coordinator

Options:
* backend (optional) - `"zookeeper"` (`"zookeeper"` by default).
* zookeeper (optional and used only if `backend` is set to `"zookeeper"`)
	* dir - Location (name without slashes) to keep state (`"rhythm"` by default).
	* addrs - Servers locations without scheme. If port is not set then default `2181` will be used (`["127.0.0.1"]` by default).
	* timeout (optional) - ZooKeeper client timeout in milliseconds (10s by default).
	* auth (optional)
		* scheme (optional) - `"digest"` or `"world"` (`"world"` by default).
		* digest (optional and used only if `scheme` is set to `"digest"`)
			* user (optional)
			* password (optional)

Example:
```javascript
"coordinator": {
    "zookeeper": {
        "addrs": ["192.168.0.1", "192.168.0.2", "192.168.0.3"],
        "dir": "rhythm",
        "timeout": 20000,
        "auth": {
            "scheme": "digest",
            "digest": {
                "user": "john",
                "password": "secret"
            }
        }
    }
}
```

### Secrets

Secrets backend allow to inject secrets into task via environment variables. Job defines secrets under `secrets` property:
```javascript
"group": "webservices",
"project": "oauth",
"id": "backup",
"secrets": {
    "DB_PASSWORD": "db/password"
}
```

Mesos task will have `DB_PASSWORD` environment variable set to value returned by secrets backend if `"webservices/oauth/db/password"` will be passed. In case of e.g. Vault it'll be interpreted as path to secret from which data under `value` key will retrieved.

Options:
* backend (optional) - `"vault"` or `"none"` (`"none"` by default).
* vault (optional and used only if `backend` is set to `"vault"`)
	* addr (required) - Vault address with scheme like `https://`.
	* token (required) - Vault token with read access to secrets under `root`.
	* root (optional) - Secret's path prefix (`"secret/rhythm/"` by defualt).
	* timeout (optional) - Client timeout in milliseconds (`0` by default which means no timeout).
	* cacert (optional) - Absolute path to CA certificate to use when verifying Vault server certificate, must be x509 PEM encoded.
    
Example:
```javascript
"secrets": {
    "backend": "vault",
    "vault": {
        "token": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaaa",
        "addr": "https://example.com"
    }
}
```

### Mesos

Options:
* addrs (required) - List of Mesos endpoints with schemes like `https://`.
* auth
	* type (optional) - `"none"` or `"basic"` (`"none"` by default).
	* basic (optional and used only if `type` is set to `"basic"`)
		* username (optional)
		* password (optional)
* cacert (optional) - Absolute path to CA certificate to use when verifying Mesos server certificate, must be x509 PEM encoded.
* checkpoint (optional) - Controls framework's checkpointing (`false` by default).
* failovertimeout (optional) - Number of milliseconds Mesos will wait for the framework to failover before killing all its tasks (7 days used by default).
* hostname (optional) - Host for which framework is registered in the Mesos Web UI.
* user (optinal) - Determine the Unix user that tasks should be launched as.
* webuiurl (optional) - Framework's Web UI address with scheme like `https://`.
* principal (optional) - Identifier used while interacting with Mesos.
* labels (optional) - Dictionary of key-value pairs assigned to framework.
* roles (optional) - List of roles framework will subscribe to (`["*"]` by default).
* logallevents (optional) - Print details of all events sent from Mesos (`false` by default).

Example:
```javascript
"mesos": {
    "addrs": ["https://example.com:5050"],
    "principal": "rhythm",
    "roles": ["rhythm"],
    "user": "root",
    "webuiurl": "https://example.com",
    "auth": {
        "type": "basic",
        "basic": {
            "username": "rhythm",
            "password": "secret"
        }
    },
    "labels": {
        "one": "1",
        "two": "2"
    }
}
```

### Logging

Logs are always sent to stderr (`level` defines verbosity) and optional backend to e.g. send certain messages to 3rd party service like Sentry. 

Options:
* level (optional)  - `"debug"`, `"info"`, `"warn"` or `"error"` (`"info"` by default).
* backend (optional) - `"sentry"` or `"none"` (`"none"` by default).
* sentry (optional and used only if `backend` is set to `"sentry"`)

    Logs with level set to warning or error will be sent to Sentry. If logging level is higher than warning then only errors will be sent (in other words `level` defines minium tier which will be by Sentry backend).
    * dsn (required) - Sentry DSN (Data Source Name) passed as string.
    * cacert (optional) - Absolute path to CA certificate to use when verifying Sentry server certificate, must be x509 PEM encoded.
    * tags (optional) - Dictionary of custom tags sent with each event.

Examples:
```javascript
"logging": {
    "level": "debug",
    "backend": "sentry",
    "sentry": {
        "dsn": "https://key@example.com/123",
        "cacert": "/var/ca.crt",
        "tags": {
            "one": "1",
            "two": "2"
        }
    }
}
```

```javascript
"logging": {
    "level": "debug"
}
```

There is `-testlogging` option which is used to test events logging. It logs sample error and then program exits. Useful to test backend like Sentry to verify that events are received.

## Command-line client
Rhythm binary besides running in server mode provides also CLI tool. To see the list of available commands use `-help`.
Address of Rhythm server is set either via `-addr` flag or `RHYTHM_ADDR` environment variable. If both are set then flag takes precedence.

### health
Provides basic information about state of the server.

Examples:

```
$ rhythm health -addr https://example.com
Leader: true
Version: 0.5
ServerTime: Mon Nov 12 20:15:49 CET 2018
```

```
$ RHYTHM_ADDR=https://example.com rhythm health
Leader: true
Version: 0.5
ServerTime: Mon Nov 12 23:53:11 CET 2018
```

### read-job
Shows configuration and state of job with the given fully-qualified ID.

Example:
```
$ rhythm read-job -addr https://example.com group/project/id
State: Idle
    Last start: Mon Nov 12 20:17:22 CET 2018
Scheduler: Cron
    Rule: */1 * * * *
    Next start: Mon Nov 12 20:18:00 CET 2018
Container: Docker
    Image: alpine:3.8
    Force pull image: false
Cmd: /bin/sh -c 'echo $FOO'
Environment:
    FOO: foo
User: someone
Resources:
    Memory: 1024.0 MB
    Disk: 0.0 MB
    CPUs: 1.0
```

### get-tasks
Shows tasks (runs) of job with the given fully-qualified ID.

Example:
```
$ rhythm get-tasks -addr https://example.com group/project/id
Status:         SUCCESS
Start:          Wed Nov 14 23:38:51 CET 2018
End:            Wed Nov 14 23:38:51 CET 2018
Task ID:        group:project:id:7eb8d4fa-f133-4880-9840-f8f62d3d06b2
Executor ID:    group:project:id:7eb8d4fa-f133-4880-9840-f8f62d3d06b2
Agent ID:       18fd8e84-9213-4f51-9343-bae8b9c517fe-S0
Framework ID:   18fd8e84-9213-4f51-9343-bae8b9c517fe-0000
Executor URL:   https://example.com:5050/#/agents/18fd8e84-9213-4f51-9343-bae8b9c517fe-S0/frameworks/18fd8e84-9213-4f51-9343-bae8b9c517fe-0000/executors/group:project:id:7eb8d4fa-f133-4880-9840-f8f62d3d06b2

Status:         FAIL
Start:          Wed Nov 14 23:41:06 CET 2018
End:            Wed Nov 14 23:41:06 CET 2018
Message:        Reading secret failed: Get https://example.com/v1/secret/rhythm/group/project/foo: dial tcp [::1]:8200: connect: connection refused
Reason:         Failed to create task
Source:         Scheduler

Status:         FAIL
Start:          Wed Nov 14 23:43:06 CET 2018
End:            Wed Nov 14 23:43:06 CET 2018
Message:        Reading secret failed: Get https://example.com/v1/secret/rhythm/group/project/foo: dial tcp [::1]:8200: connect: connection refused
Reason:         Failed to create task
Source:         Scheduler

Status:         FAIL
Start:          Wed Nov 14 23:51:04 CET 2018
End:            Wed Nov 14 23:51:04 CET 2018
Task ID:        group:project:id:504d8c5c-f4d1-41f6-8797-c608bee7195b
Executor ID:    group:project:id:504d8c5c-f4d1-41f6-8797-c608bee7195b
Agent ID:       18fd8e84-9213-4f51-9343-bae8b9c517fe-S0
Framework ID:   18fd8e84-9213-4f51-9343-bae8b9c517fe-0000
Executor URL:   https://example.com:5050/#/agents/18fd8e84-9213-4f51-9343-bae8b9c517fe-S0/frameworks/18fd8e84-9213-4f51-9343-bae8b9c517fe-0000/executors/group:project:id:504d8c5c-f4d1-41f6-8797-c608bee7195b
Message:        Container exited with status 127
Reason:         REASON_COMMAND_EXECUTOR_FAILED
Source:         SOURCE_EXECUTOR
```

### delete-job
Remove job with the given fully-qualified ID.

Example:
```
$ rhythm delete-job -addr https://example.com group/project/id
```

### create-job
Add new job specified by config file.

Example:
```
$ rhythm create-job --addr=https://example.com echo.json
```

echo.json:
```javascript
{
    "id": "id",
    "group": "group",
    "project": "project",
    "cpus": 1,
    "mem": 1024,
    "cmd": "echo $FOO",
    "user": "someone",
    "env": {
        "FOO": "bar"
    },
    "schedule": {
        "cron": "*/1 * * * *"
    },
    "container": {
        "docker": {
            "image": "alpine:3.8"
        }
    }
}
```

### update-job
Modify job specified by given fully-qualified ID (e.g. "group/project/id") with config file containing parameters to change.
Only parameters form config file will be changed - absent parameters wont' be modified.

Example:
```
rhythm update-job --addr=https://example.com group/project/id diff.json
```

diff.json:
```javascript
{
    "user": "root"
}
```

### get-jobs
Show IDs of jobs matching FILTER.

FILTER can be one of:
* GROUP to return all jobs from group
* GROUP/PROJECT to return all jobs from project
* no set to return all jobs across all groups and projects

Examples:
```
rhythm get-jobs --addr=https://example.com group
group:project:id
group:project:id2
group:project2:id
```

```
rhythm get-jobs --addr=https://example.com group/project
group:project:id
group:project:id2
```

```
rhythm get-jobs --addr=https://example.com
group:project:id
group:project:id2
group:project2:id
group2:project:id
```
