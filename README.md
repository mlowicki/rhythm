# Rhythm

## Features

* Support for [Docker](https://mesos.apache.org/documentation/latest/docker-containerizer/) and [Mesos](https://mesos.apache.org/documentation/latest/mesos-containerizer/) Containerizers 
* Integration with [HashiCorp Vault](https://www.vaultproject.io/) for secrets management
* Access control list (ACL) backed by [GitLab](https://gitlab.com/)
* [Cron syntax](http://www.nncron.ru/help/EN/working/cron-format.htm)
* Integration with [Sentry](https://sentry.io/) for error tracking

## API

[Documentation](https://mlowicki.github.io/rhythm/api)

## Configuration

Rhythm is configured using file in JSON format. By default config.json from current  directory is used but it can overwritten using `-config` parameter.
There are couple of sections in configuration file:
* api
* storage
* coordinator
* secrets
* mesos
* logging

### API

Options:
* addr (optional) - Address (without scheme) API server will bind to. If `certfile` or `keyfile` is set then HTTPS will be used, otherwise HTTP (`"localhost:8000"` by default)
* certfile (optional) - absolute path to certificate
* keyfile (optional) - absolute path to private key
* auth (optional)
    * backend (optional) - `"none"` or `"gitlab"` (`"none"` by default)
    * gitlab (optional and used only if `backend` is set to `"gitlab"`)
        * addr (required) - GitLab address with scheme like `https://`
        * cacert (optional) - absolute path to CA certificate to use when verifying GitLab server certificate, must be x509 PEM encoded.

Example:
```javascript
"api": {
    "addr": "localhost:8888",
    "auth": {
        "backend": "gitlab",
        "gitlab": {
            "addr": "https://example.com",
            "cacert": "/var/ca.crt",
        }
    }
}
```

### Storage

Options:
* backend (optional) - `"zookeeper"` (`"zookeeper"` by default)
* zookeeper (optional and used only if `backend` is set to `"zookeeper"`)
    * dir - location (name without slashes) to store data (`"rhythm"` by default)
    * addrs - servers locations without scheme. If port is not set then default `2181` will be used (`["127.0.0.1"]` by default)
    * timeout (optional) - ZooKeeper client timeout in milliseconds (10s by default)
    * auth (optional)
        * scheme (optional) - `"digest"` or `"world"` (`"world"` by default)
        * digest (optional and used only if `scheme` is set to `"digest"`)
            * user (optional)
            * password (optional)

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
* backend (optional) - `"zookeeper"` (`"zookeeper"` by default)
* zookeeper (optional and used only if `backend` is set to `"zookeeper"`)
    * dir - location (name without slashes) to keep state (`"rhythm"` by default)
    * addrs - servers locations without scheme. If port is not set then default `2181` will be used (`["127.0.0.1"]` by default)
    * timeout (optional) - ZooKeeper client timeout in milliseconds (10s by default)
    * auth (optional)
		* scheme (optional) - `"digest"` or `"world"` (`"world"` by default)
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
* backend (optional) - `"vault"` or `"none"` (`"none"` by default)
* vault (optional and used only if `backend` is set to `"vault"`)
    * addr (required) - Vault address with scheme like `https://`
    * token (required) - Vault token with read access to secrets under `root`
    * root (optional) - Secret's path prefix (`"secret/rhythm/"` by defualt)
    * timeout (optional) - Client timeout in milliseconds (`0` by default which means no timeout)
    * cacert (optional) - absolute path to CA certificate to use when verifying Vault server certificate, must be x509 PEM encoded.
    
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
* addrs (required) - list of Mesos endpoints with schemes like `https://`
* auth
    * type (optional) - `"none"` or `"basic"` (`"none"` by default)
    * basic (optional and used only if `type` is set to `"basic"`)
        * username (optional)
        * password (optional)
* cacert (optional) - absolute path to CA certificate to use when verifying Mesos server certificate, must be x509 PEM encoded.
* checkpoint (optional) - controls framework's checkpointing (`false` by default)
* failovertimeout (optional) - number of milliseconds Mesos will wait for the framework to failover before killing all its tasks (7 days used by default)
* hostname (optional) - host for which framework is registered in the Mesos Web UI
* user (optinal) - determine the Unix user that tasks should be launched as
* webuiurl (optional) - framework's Web UI address with scheme like `https://`
* principal (optional) - identifier used while interacting with Mesos
* labels (optional) - dictionary of key-value pairs assigned to framework
* roles (optional) - list of roles framework will subscribe to (`["*"]` by default)
* logallevents (optional) - print details of all events sent from Mesos (`false` by default)

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
* level (optional)  - `"debug"`, `"info"`, `"warn"` or `"error"` (`"info"` by default)
* backend (optional) - `"sentry"` or `"none"` (`"none"` by default)
* sentry (optional and used only if `backend` is set to `"sentry"`)

    Logs with level set to warning or error will be sent to Sentry. If logging level is higher than warning then only errors will be sent (in other words `level` defines minium tier which will be by Sentry backend).
    * dsn (required) - Sentry DSN (Data Source Name) passed as string
    * cacert (optional) - absolute path to CA certificate to use when verifying Sentry server certificate, must be x509 PEM encoded.
    * tags (optional) - dictionary of custom tags sent with each event

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
