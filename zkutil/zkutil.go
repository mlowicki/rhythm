package zkutil

import (
	"fmt"

	"github.com/mlowicki/rhythm/conf"
	"github.com/samuel/go-zookeeper/zk"
)

func AddAuth(conn *zk.Conn, c *conf.ZKAuth) (func(perms int32) []zk.ACL, error) {
	if c.Scheme == conf.ZKAuthSchemeDigest {
		err := conn.AddAuth("digest", []byte(c.Digest.User+":"+c.Digest.Password))
		return zk.AuthACL, err
	} else if c.Scheme == conf.ZKAuthSchemeWorld {
		return zk.WorldACL, nil
	}
	return nil, fmt.Errorf("Unknown auth scheme: %s", c.Scheme)
}
