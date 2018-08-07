# Rhythm

It's an early prototype - not ready to run on production.

Features:
* Support for [Docker](https://www.docker.com/)
* Integration with [HashiCorp Vault](https://www.vaultproject.io/)
* Access control list (ACL) backed by [GitLab](https://gitlab.com/)

TODO
* Support for [Mesos Containerizer](https://mesos.apache.org/documentation/latest/mesos-containerizer/)
* Integration with [Sentry](https://sentry.io/)

## Development environment

Run [HashiCorp Vault](https://www.vaultproject.io/):
```
docker run --cap-add=IPC_LOCK -d -p 127.0.0.1:8200:8200 -e 'VAULT_DEV_ROOT_TOKEN_ID=token' vault:0.9.1
```

Run [Apache ZooKeeper](https://zookeeper.apache.org/):
```
docker run -p 2181:2181 -v=data:/data -v=datalog:/datalog zookeeper
```

[Build Mesos](https://mesos.apache.org/documentation/latest/building/) and run both master and agent:
```
MESOS_DIR/build/bin/mesos-master.sh --ip=127.0.0.1 --work_dir=/tmp/mesos
MESOS_DIR/mesos/build/bin/mesos-agent.sh --master=127.0.0.1:5050 --work_dir=/tmp/mesos --containerizers=docker
```

Run *rhythm*:
```
go run *.go
```
