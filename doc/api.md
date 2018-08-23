# API

## Authorization

### GitLab

[Personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) from GitLab with `api` scope should be passed via `X-Token` HTTP header.

`curl -H "X-Token: TOKEN" -X GET API_ADDRESS/v1/jobs/GROUP/PROJECT/ID`

## Jobs management

### Get all jobs
`curl -v API_ADDRESS/v1/jobs`

### Get all jobs from specified group
`curl -v API_ADDRESS/v1/jobs/GROUP`

### Get all jobs from specified group and project
`curl -v API_ADDRESS/v1/jobs/GROUP/PROJECT`

### Get job
`curl -v API_ADDRESS/v1/jobs/GROUP/PROJECT/ID`

### Create job
`curl -v -X POST -d '{"id": "id", "group": "group", "project": "project", "cpus": 4, "mem": 7168, "cmd": "echo $FOO", "user": "user", "env": {"FOO": "foo"}, "schedule": {"cron": "*/1 * * * *"}, "container": {"docker": {"image": "alpine:3.8"}}}' API_ADDRESS/v1/jobs`

### Update job
`curl -v -X PUT API_ADDRESS/v1/jobs/GROUP/PROJECT/ID -d '{"schedule": {"cron": "*/2 * * * *"}}'`

### Delete job
`curl -v -X DELETE API_ADDRESS/v1/jobs/GROUP/PROJECT/ID`
