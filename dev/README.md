# Development environment

To run Mesos, ZooKeeper and Vault:
```
cd dev
docker-compose up
```

Run *rhythm*:
```
go run *.go -config dev/server.json
```

## API over HTTPS

Self-signed certificate and key are generated in /dev (server.key & server.csr).
They've been generated based on https://devcenter.heroku.com/articles/ssl-certificate-self but with `-days 3650`.


1. Set `api.certfile` and `api.keyfile` in server.json to absolute paths pointing to cert and key in /dev.
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
