# Rhythm

## Features

* Support for [Docker Containerizer](https://mesos.apache.org/documentation/latest/docker-containerizer/)
* Support for [Mesos Containerizer](https://mesos.apache.org/documentation/latest/mesos-containerizer/)
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

TODO

### Storage

TODO

### Coordinator

TODO

### Secrets

Secrets backend allow to inject secrets into task via environment variables. Job defines secrets under `secrets` property:
```json
"group": "webservices",
"project": "oauth",
"id": "backup",
"secrets": {
    "DB_PASSWORD": "db/password"
}
```

Mesos task will have "DB_PASSWORD" environment variable set to value returned by secrets backend when "webservices/oauth/db/password" will be passed. In case of e.g. Vault it'll be interpreted as path to secret.

Options:
* backend (optional) - "vault" or "none" ("none" used by default)
* vault (optional and used only when "backend" is set to "vault")
    * address (required) - Vault server address
    * token (required) - Vault token with read access to secrets under `root`
    * root (optional) - Secret's path prefix ("secret/rhythm/" used by defualt)
    * timeout (optional) - Client timeout in seconds (0 used by default which means no timeout)
    * rootca (optional) - absolute path to custom root certificate used while talking to Vault server
    
Example:
```json
    "secrets": {
        "backend": "vault",
        "vault": {
            "token": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaaa",
            "address": "https://example.com"
        }
    }
```

### Mesos

TODO

### Logging

Logs are always sent to stderr (`level` defines verbosity) and optional backend to e.g. send certain messages to 3rd party service like Sentry. 

Options:
* level (optional)  - "debug", "info", "warn" or "error" ("info" used by default)
* backend (optional) - "sentry" or "none" ("none" used by default)
* sentry (optional and used only when "backend" is set to "sentry")

    Logs with level set to warning or error will be sent to Sentry. If logging level is higher than warning then only errors will be sent (in other words `level` defines minium tier which will be used by Sentry backend).
    * dsn (required) - Sentry DSN (Data Source Name) passed as string
    * rootca (optional) - absolute path to custom root certificate used while talking to Sentry server
    * tags (optional) - dictionary of custom tags sent with each event

Examples:
```json
    "logging": {
        "level": "debug",
        "backend": "sentry",
        "sentry": {
            "dsn": "https://key@example.com/123",
            "rootca": "/var/rootca.crt",
            "tags": {
                "one": "1",
                "two": "2"
            }
        }
    }
```

```json
    "logging": {
        "level": "debug"
    }
```

There is `-testlogging` option which is used to test events logging. It logs sample error and then program exits. Useful to test backend like Sentry to verify that events are received.
