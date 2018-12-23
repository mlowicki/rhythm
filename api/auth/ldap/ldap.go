package ldap

// Heavily inspired by https://github.com/hashicorp/vault/blob/bc33dbd/helper/ldaputil/client.go
/// or simply copying code from there.

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/go-ldap/ldap"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/vault/helper/ldaputil"
	"github.com/mlowicki/rhythm/api/auth"
	"github.com/mlowicki/rhythm/conf"
	tlsutils "github.com/mlowicki/rhythm/tls"
	log "github.com/sirupsen/logrus"
)

type Authorizer struct {
	addrs              []string
	userDN             string
	userAttr           string
	caCert             *x509.CertPool
	userACL            map[string]map[string]string
	groupACL           map[string]map[string]string
	bindDN             string
	bindPassword       string
	groupFilter        string
	groupDN            string
	groupAttr          string
	caseSensitiveNames bool
}

const (
	READONLY  = "readonly"
	READWRITE = "readwrite"
)

var timeoutMut sync.Mutex

// This function sets package-level variable in github.com/go-ldap/ldap.
func SetTimeout(timeout time.Duration) {
	timeoutMut.Lock()
	ldap.DefaultTimeout = timeout
	timeoutMut.Unlock()
}

func (a *Authorizer) dial() (*ldap.Conn, error) {
	var errs *multierror.Error
	var conn *ldap.Conn
	for _, addr := range a.addrs {
		u, err := url.Parse(addr)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Error parsing URL: %s", err))
			continue
		}
		host, port, err := net.SplitHostPort(u.Host)
		if err != nil {
			host = u.Host
		}
		switch u.Scheme {
		case "ldap":
			if port == "" {
				port = "389"
			}
			conn, err = ldap.Dial("tcp", net.JoinHostPort(host, port))
			if err != nil {
				break
			}
		case "ldaps":
			tlsConfig := &tls.Config{
				ServerName: host,
			}
			if a.caCert != nil {
				tlsConfig.RootCAs = a.caCert
			}
			if port == "" {
				port = "636"
			}
			conn, err = ldap.DialTLS("tcp", net.JoinHostPort(host, port), tlsConfig)
			if err != nil {
				break
			}
		default:
			errs = multierror.Append(errs, fmt.Errorf("Invalid LDAP scheme: %s", u.Scheme))
			continue
		}
		if err == nil {
			if errs != nil {
				log.Debug(errs.Error())
			}
			errs = nil
			break
		}
		errs = multierror.Append(errs, err)
	}
	return conn, errs.ErrorOrNil()
}

func (a *Authorizer) getUserBindDN(conn *ldap.Conn, username string) (string, error) {
	bindDN := ""
	if a.bindDN != "" && a.bindPassword != "" {
		err := conn.Bind(a.bindDN, a.bindPassword)
		if err != nil {
			return bindDN, fmt.Errorf("LDAP service bind failed: %s", err)
		}
		filter := fmt.Sprintf("(%s=%s)", a.userAttr, ldap.EscapeFilter(username))
		res, err := conn.Search(&ldap.SearchRequest{
			BaseDN:    a.userDN,
			Scope:     ldap.ScopeWholeSubtree,
			Filter:    filter,
			SizeLimit: math.MaxInt32,
		})
		if err != nil {
			return bindDN, fmt.Errorf("LDAP search for binddn failed: %s", err)
		}
		if len(res.Entries) != 1 {
			return bindDN, errors.New("LDAP search for binddn not found or not unique")
		}
		bindDN = res.Entries[0].DN
	} else {
		bindDN = fmt.Sprintf("%s=%s,%s", a.userAttr, ldaputil.EscapeLDAPValue(username), a.userDN)
	}
	return bindDN, nil
}

func (a *Authorizer) performLDAPGroupsSearch(conn *ldap.Conn, userDN, username string) ([]*ldap.Entry, error) {
	t, err := template.New("queryTemplate").Parse(a.groupFilter)
	if err != nil {
		return nil, fmt.Errorf("Failed compilation of LDAP query template: %s", err)
	}
	context := struct {
		UserDN   string
		Username string
	}{
		ldap.EscapeFilter(userDN),
		ldap.EscapeFilter(username),
	}
	var renderedQuery bytes.Buffer
	t.Execute(&renderedQuery, context)
	log.Debugf("Groups search query: %s", renderedQuery.String())
	res, err := conn.Search(&ldap.SearchRequest{
		BaseDN: a.groupDN,
		Scope:  ldap.ScopeWholeSubtree,
		Filter: renderedQuery.String(),
		Attributes: []string{
			a.groupAttr,
		},
		SizeLimit: math.MaxInt32,
	})
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %s", err)
	}
	return res.Entries, nil
}

func (a *Authorizer) getLDAPGroups(conn *ldap.Conn, userDN, username string) ([]string, error) {
	entries, err := a.performLDAPGroupsSearch(conn, userDN, username)
	groupsMap := make(map[string]bool)
	for _, e := range entries {
		dn, err := ldap.ParseDN(e.DN)
		if err != nil || len(dn.RDNs) == 0 {
			continue
		}
		values := e.GetAttributeValues(a.groupAttr)
		if len(values) > 0 {
			for _, val := range values {
				groupCN := getCN(val)
				groupsMap[groupCN] = true
			}
		} else {
			// If groupattr didn't resolve, use self (enumerating group objects)
			groupCN := getCN(e.DN)
			groupsMap[groupCN] = true
		}
	}
	groups := make([]string, 0, len(groupsMap))
	for key := range groupsMap {
		groups = append(groups, key)
	}
	return groups, err
}

/*
 * Parses a distinguished name and returns the CN portion.
 * Given a non-conforming string (such as an already-extracted CN), it will be returned as-is.
 */
func getCN(dn string) string {
	parsedDN, err := ldap.ParseDN(dn)
	if err != nil || len(parsedDN.RDNs) == 0 {
		// It was already a CN, return as-is
		return dn
	}
	for _, rdn := range parsedDN.RDNs {
		for _, rdnAttr := range rdn.Attributes {
			if rdnAttr.Type == "CN" {
				return rdnAttr.Value
			}
		}
	}
	// Default, return self
	return dn
}

func (a *Authorizer) GetProjectAccessLevel(r *http.Request, group string, project string) (auth.AccessLevel, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return auth.NoAccess, errors.New("Cannot parse Basic auth credentials")
	}
	conn, err := a.dial()
	if err != nil {
		return auth.NoAccess, err
	}
	defer conn.Close()
	bindDN, err := a.getUserBindDN(conn, username)
	log.Debugf("bindDN: %s", bindDN)
	if err != nil {
		return auth.NoAccess, err
	}
	if len(password) > 0 {
		err = conn.Bind(bindDN, password)
	} else {
		err = conn.UnauthenticatedBind(bindDN)
	}
	if err != nil {
		return auth.NoAccess, err
	}
	userLevel := a.getLevelFromACL(&a.userACL, username, group, project)
	if userLevel != auth.NoAccess {
		return userLevel, nil
	}
	ldapGroups, err := a.getLDAPGroups(conn, bindDN, username)
	log.Debugf("Found groups: %v", ldapGroups)
	if err != nil {
		return auth.NoAccess, err
	}
	readOnlyAccess := false
	for _, ldapGroup := range ldapGroups {
		groupLevel := a.getLevelFromACL(&a.groupACL, ldapGroup, group, project)
		if groupLevel == auth.ReadWrite {
			return auth.ReadWrite, nil
		}
		if groupLevel == auth.ReadOnly {
			readOnlyAccess = true
		}
	}
	if readOnlyAccess {
		return auth.ReadOnly, nil
	}
	return auth.NoAccess, nil
}

func (a *Authorizer) getLevelFromACL(acl *map[string]map[string]string, name, group, project string) auth.AccessLevel {
	if !a.caseSensitiveNames {
		name = strings.ToLower(name)
	}
	rules, ok := (*acl)[name]
	if !ok {
		return auth.NoAccess
	}
	keys := [...]string{
		fmt.Sprintf("%s/%s", group, project),
		group,
		"*",
	}
	for _, key := range keys {
		level, ok := rules[key]
		if !ok {
			continue
		}
		switch level {
		case READONLY:
			return auth.ReadOnly
		case READWRITE:
			return auth.ReadWrite
		}
	}
	return auth.NoAccess
}

func validateACL(acl *map[string]map[string]string) error {
	for outerKey, rules := range *acl {
		for innerKey, level := range rules {
			if level != READONLY && level != READWRITE {
				return fmt.Errorf("invalid access level [%s][%s]: %s", outerKey, innerKey, level)
			}
		}
	}
	return nil
}

func New(c *conf.APIAuthLDAP) (*Authorizer, error) {
	if len(c.Addrs) == 0 {
		return nil, errors.New("addrs is empty")
	}
	if c.UserDN == "" {
		return nil, errors.New("userdn is empty")
	}
	if c.UserAttr == "" {
		return nil, errors.New("userattr is empty")
	}
	if c.GroupDN == "" {
		return nil, errors.New("groupdn is empty")
	}
	if c.GroupAttr == "" {
		return nil, errors.New("groupattr is empty")
	}
	if c.GroupFilter == "" {
		return nil, errors.New("groupfilter is empty")
	}
	err := validateACL(&c.UserACL)
	if err != nil {
		return nil, fmt.Errorf("Invalid useracl: %s", err)
	}
	err = validateACL(&c.GroupACL)
	if err != nil {
		return nil, fmt.Errorf("Invalid groupacl: %s", err)
	}
	a := Authorizer{
		addrs:              c.Addrs,
		userDN:             c.UserDN,
		userAttr:           c.UserAttr,
		userACL:            c.UserACL,
		groupACL:           c.GroupACL,
		bindDN:             c.BindDN,
		bindPassword:       c.BindPassword,
		groupFilter:        c.GroupFilter,
		groupDN:            c.GroupDN,
		groupAttr:          c.GroupAttr,
		caseSensitiveNames: c.CaseSensitiveNames,
	}
	if c.CACert != "" {
		pool, err := tlsutils.BuildCertPool(c.CACert)
		if err != nil {
			return nil, err
		}
		a.caCert = pool
	}
	return &a, nil
}
