# Development environment

Run [HashiCorp Vault](https://www.vaultproject.io/):
```
docker run --cap-add=IPC_LOCK -d -p 127.0.0.1:8200:8200 -e 'VAULT_DEV_ROOT_TOKEN_ID=token' vault:0.9.1
```

Run [Apache ZooKeeper](https://zookeeper.apache.org/):
```
docker run -p 2181:2181 -v=/tmp/zookeeper/data:/data -v=/tmp/zookeeper/datalog:/datalog zookeeper
```

[Build Mesos](https://mesos.apache.org/documentation/latest/building/) and run both master and agent:
```
MESOS_DIR/build/bin/mesos-master.sh --ip=127.0.0.1 --work_dir=/tmp/mesos/master --authenticate_http_frameworks --http_framework_authenticators=basic --credentials=development/credentials
MESOS_DIR/mesos/build/bin/mesos-agent.sh --master=127.0.0.1:5050 --work_dir=/tmp/mesos/agent --containerizers=docker
```

Run *rhythm*:
```
go run *.go --config dev/config.json
```

## API over HTTPS

Self-signed certificate and key are generated in /dev (server.key & server.csr).
They've been generated based on https://devcenter.heroku.com/articles/ssl-certificate-self but with `-days 3650`.


1. Set `api.certfile` and `api.keyfile` in config.json to absolute paths pointing to cert and key in /dev.
2. Add `127.0.0.1 rhythm` to `/etc/hosts`
3. Run *rhythm*
4. `curl curl -v --cacert dev/server.crt https://rhythm:8000/api/v1/jobs/group/project/id`

## LDAP

1. `docker run --name my-openldap-container --detach -p 389:389 osixia/openldap:1.2.2` ([Docker image for OpenLDAP](https://github.com/osixia/docker-openldap))
2. Changes to LDAP server database can be made with `docker exec -ti my-openldap-container ldapmodify -D "cn=admin,dc=example,dc=org" -w admin`
3. Enable LDAP backend:
```javascript
"api": {
    "auth": {
        "backend": "ldap",
        "ldap": {
            "addrs": ["ldap://localhost"],
            "userdn": "dc=example,dc=org",
            "userattr": "cn",
            "groupdn": "ou=Groups,dc=example,dc=org",
            "useracl": {
                "admin": {
                     "group/project": "readwrite"
                }
            }
        }
    }
}
```
