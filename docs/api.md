# API

## Create job

```
curl -v -X POST -d '{"id": "id", "group": "group", "project": "project", "cpus": 4, "mem": 7168, "cmd": "echo $FOO", "user": "user", "env": {"FOO": "foo"}, "schedule": {"cron": "*/1 * * * *"}, "container": {"docker": {"image": "alpine:3.8"}}}' API_ADDRESS/api/v1/jobs
```

## Update job

```
curl -v -X PUT API_ADDRESS/api/v1/jobs/GROUP/PROJECT/ID -d '{"schedule": {"cron": "*/2 * * * *"}}'
```
