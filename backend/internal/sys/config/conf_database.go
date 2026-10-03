package config

import (
	"encoding/json"
	"path"
	"strings"
)

const (
	DriverSqlite3  = "sqlite3"
	DriverPostgres = "postgres"
)

type Storage struct {
	// Data is the path to the root directory
	PrefixPath string `yaml:"prefix_path" conf:"default:.data"`
	ConnString string `yaml:"conn_string" conf:"default:file:///./"`
}

func (s Storage) MarshalJSON() ([]byte, error) {
	type alias Storage
	a := alias(s)
	a.ConnString = redactURLUserinfo(a.ConnString)
	return json.Marshal(a)
}

// withoutRootBucket rewrites a file:// bucket rooted at the filesystem root
// ("file:///" + prefix "data") into an equivalent bucket rooted at the prefix
// directory ("file:///data" + prefix ""). gocloud.dev/blob/fileblob v0.46.0
// rejects every key with "escapes bucket root" when the bucket dir is "/",
// because its containment check compares against dir+"/" ("//"). The on-disk
// location (/data/<gid>/documents/<hash>) and the keys stored in the database
// are unchanged, so existing deployments keep working without config edits.
func (s Storage) withoutRootBucket() Storage {
	const scheme = "file://"
	if !strings.HasPrefix(s.ConnString, scheme) {
		return s
	}

	rest := strings.TrimPrefix(s.ConnString, scheme)
	dirPart, query := rest, ""
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		dirPart, query = rest[:i], rest[i:]
	}
	if dirPart != "/" {
		return s
	}

	prefix := strings.Trim(strings.ReplaceAll(s.PrefixPath, `\`, "/"), "/")
	if prefix == "" {
		return s
	}
	dir := path.Clean("/" + prefix)
	if dir == "/" {
		return s
	}

	s.ConnString = scheme + dir + query
	s.PrefixPath = ""
	return s
}

type Database struct {
	Driver           string `yaml:"driver"             conf:"default:sqlite3"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	Host             string `yaml:"host"`
	Port             string `yaml:"port"`
	Database         string `yaml:"database"`
	SslMode          string `yaml:"ssl_mode"           conf:"default:require"`
	SslRootCert      string `yaml:"ssl_rootcert"`
	SslCert          string `yaml:"ssl_cert"`
	SslKey           string `yaml:"ssl_key"`
	SqlitePath       string `yaml:"sqlite_path"        conf:"default:./.data/homebox.db?_pragma=busy_timeout=999&_pragma=journal_mode=WAL&_fk=1&_time_format=sqlite"`
	PubSubConnString string `yaml:"pubsub_conn_string" conf:"default:mem://{{ .Topic }}"`
}

func (d Database) MarshalJSON() ([]byte, error) {
	type alias Database
	a := alias(d)
	if a.Password != "" {
		a.Password = redactedValue
	}
	a.PubSubConnString = redactURLUserinfo(a.PubSubConnString)
	return json.Marshal(a)
}
