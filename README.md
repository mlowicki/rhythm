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

TODO

### Mesos

TODO

### Logging

Options:
* level (optional)  - "debug", "info", "warn" or "error" ("info" used by default)
* backend (optional) - "sentry" or "none"
* sentry (optional and used only when "backend" is set to "sentry")
    * dsn (required) - Sentry DSN (Data Source Name) passed as string
    * rootca (optional) - absolute path to custom root certificate used while talking to Sentry server
    * tags (optional) - dictionary of custom tags sent with each event

Example:
```
    {
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
    }
```
