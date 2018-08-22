# API

## Authorization

TBD

## Jobs management

### Get all jobs
curl -v API_ADDRESS/v1/jobs

### Get all jobs from specified group
curl -v API_ADDRESS/v1/jobs/GROUP

### Get all jobs from specified group and project
curl -v API_ADDRESS/v1/jobs/GROUP/PROJECT

### Get job
curl -v API_ADDRESS/v1/jobs/GROUP/PROJECT/ID

### Create job
curl -v -X POST -d '{"id": "foo", "z": "x", "project": "y", "cpus": 4, "mem": 7168, "cmd": "echo $BAR", "user": "mlowicki", "env": {"BAR": "bar", "FOO": "foo"}, "schedule": {"cron": "*    /1 * * * *"}, "container": {"docker": {"image": "alpine:3.8"}}}' API_ADDRESS/v1/jobs

### Update job
curl -v -X PUT API_ADDRESS/v1/jobs/GROUP/PROJECT/ID -d '{"schedule": {"cron": "*/2 * * * *"}}'

### Delete job
curl -v -X DELETE API_ADDRESS/v1/jobs/GROUP/PROJECT/ID
