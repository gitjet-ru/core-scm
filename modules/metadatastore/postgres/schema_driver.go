// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package postgres

import (
	"database/sql"
	"database/sql/driver"
	"sync"

	"github.com/gitjet-ru/core-scm/modules/setting"

	"github.com/lib/pq"
	"xorm.io/xorm/dialects"
)

var registerOnce sync.Once

// EnsureSchemaDriverRegistered registers the postgresschema driver when [database] SCHEMA is set.
func EnsureSchemaDriverRegistered() {
	if len(setting.Database.Schema) == 0 {
		return
	}
	registerPostgresSchemaDriver()
}

func registerPostgresSchemaDriver() {
	registerOnce.Do(func() {
		sql.Register("postgresschema", &postgresSchemaDriver{})
		dialects.RegisterDriver("postgresschema", dialects.QueryDriver("postgres"))
	})
}

type postgresSchemaDriver struct {
	pq.Driver
}

// Open opens a new connection to the database. name is a connection string.
// This function opens the postgres connection in the default manner but immediately
// runs set_config to set the search_path appropriately
func (d *postgresSchemaDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return conn, err
	}
	schemaValue, _ := driver.String.ConvertValue(setting.Database.Schema)

	// golangci lint is incorrect here - there is no benefit to using driver.ExecerContext here
	// and in any case pq does not implement it
	if execer, ok := conn.(driver.Execer); ok { //nolint:staticcheck // see above
		_, err := execer.Exec(`SELECT set_config(
			'search_path',
			$1 || ',' || current_setting('search_path'),
			false)`, []driver.Value{schemaValue})
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	}

	stmt, err := conn.Prepare(`SELECT set_config(
		'search_path',
		$1 || ',' || current_setting('search_path'),
		false)`)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer stmt.Close()

	// golangci lint is incorrect here - there is no benefit to using stmt.ExecWithContext here
	_, err = stmt.Exec([]driver.Value{schemaValue}) //nolint:staticcheck // see above
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}
