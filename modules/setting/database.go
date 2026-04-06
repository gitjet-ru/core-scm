// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gitjet-ru/core-scm/modules/log"
)

var (
	// SupportedDatabaseTypes is the list of supported metadata database drivers (GitJet: PostgreSQL only).
	SupportedDatabaseTypes = []string{"postgres"}
	// DatabaseTypeNames contains the friendly names for database types
	DatabaseTypeNames = map[string]string{"postgres": "PostgreSQL"}

	// Database holds the database settings
	Database = struct {
		Type               DatabaseType
		Host               string
		Name               string
		User               string
		Passwd             string
		Schema             string
		SSLMode            string
		Path               string // legacy [database] PATH in app.ini; unused for PostgreSQL
		LogSQL             bool
		CharsetCollation   string
		DBConnectRetries   int
		DBConnectBackoff   time.Duration
		MaxIdleConns       int
		MaxOpenConns       int
		ConnMaxLifetime    time.Duration
		IterateBufferSize  int
		AutoMigration      bool
		SlowQueryThreshold time.Duration
	}{
		IterateBufferSize: 50,
	}
)

// LoadDBSetting loads the database settings
func LoadDBSetting() {
	loadDBSetting(CfgProvider)
}

func loadDBSetting(rootCfg ConfigProvider) {
	sec := rootCfg.Section("database")
	Database.Type = DatabaseType(sec.Key("DB_TYPE").String())
	if Database.Type == "" {
		Database.Type = "postgres"
	}
	if Database.Type != "postgres" {
		log.Fatal("GitJet requires [database] DB_TYPE=postgres, got %q", Database.Type.String())
	}

	Database.Host = sec.Key("HOST").String()
	Database.Name = sec.Key("NAME").String()
	Database.User = sec.Key("USER").String()
	if len(Database.Passwd) == 0 {
		Database.Passwd = sec.Key("PASSWD").String()
	}
	Database.Schema = sec.Key("SCHEMA").String()
	Database.SSLMode = sec.Key("SSL_MODE").MustString("disable")
	Database.Path = sec.Key("PATH").String()
	Database.CharsetCollation = sec.Key("CHARSET_COLLATION").String()

	Database.MaxIdleConns = sec.Key("MAX_IDLE_CONNS").MustInt(2)
	Database.ConnMaxLifetime = sec.Key("CONN_MAX_LIFETIME").MustDuration(0)
	Database.MaxOpenConns = sec.Key("MAX_OPEN_CONNS").MustInt(0)

	Database.IterateBufferSize = sec.Key("ITERATE_BUFFER_SIZE").MustInt(50)
	Database.LogSQL = sec.Key("LOG_SQL").MustBool(false)
	Database.DBConnectRetries = sec.Key("DB_RETRIES").MustInt(10)
	Database.DBConnectBackoff = sec.Key("DB_RETRY_BACKOFF").MustDuration(3 * time.Second)
	Database.AutoMigration = sec.Key("AUTO_MIGRATION").MustBool(true)
	Database.SlowQueryThreshold = sec.Key("SLOW_QUERY_THRESHOLD").MustDuration(5 * time.Second)
}

// DBConnStr returns database connection string (PostgreSQL only).
func DBConnStr() (string, error) {
	return getPostgreSQLConnectionString(Database.Host, Database.User, Database.Passwd, Database.Name, Database.SSLMode), nil
}

// parsePostgreSQLHostPort parses given input in various forms defined in
// https://www.postgresql.org/docs/current/static/libpq-connect.html#LIBPQ-CONNSTRING
// and returns proper host and port number.
func parsePostgreSQLHostPort(info string) (host, port string) {
	if h, p, err := net.SplitHostPort(info); err == nil {
		host, port = h, p
	} else {
		// treat the "info" as "host", if it's an IPv6 address, remove the wrapper
		host = info
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
		}
	}

	// set fallback values
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "5432"
	}
	return host, port
}

func getPostgreSQLConnectionString(dbHost, dbUser, dbPasswd, dbName, dbsslMode string) (connStr string) {
	dbName, dbParam, _ := strings.Cut(dbName, "?")
	host, port := parsePostgreSQLHostPort(dbHost)
	connURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(dbUser, dbPasswd),
		Host:     net.JoinHostPort(host, port),
		Path:     dbName,
		OmitHost: false,
		RawQuery: dbParam,
	}
	query := connURL.Query()
	if strings.HasPrefix(host, "/") { // looks like a unix socket
		query.Add("host", host)
		connURL.Host = ":" + port
	}
	query.Set("sslmode", dbsslMode)
	connURL.RawQuery = query.Encode()
	return connURL.String()
}

type DatabaseType string

func (t DatabaseType) String() string {
	return string(t)
}

func (t DatabaseType) IsPostgreSQL() bool {
	return t == "postgres"
}

// MustBePostgreSQL returns an error if the database type is not postgres.
func (t DatabaseType) MustBePostgreSQL() error {
	if t != "postgres" {
		return fmt.Errorf("unsupported database type %q: GitJet requires postgres", t)
	}
	return nil
}
