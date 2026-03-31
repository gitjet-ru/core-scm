// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package postgres

import (
	"context"
	"fmt"

	"github.com/gitjet-ru/core-scm/models/db"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

// Backend is the PostgreSQL metadata store implementation (xorm + lib/pq).
type Backend struct{}

// New returns a PostgreSQL metadata Backend.
func New() *Backend {
	return &Backend{}
}

func newXORMEngine() (*xorm.Engine, error) {
	connStr, err := setting.DBConnStr()
	if err != nil {
		return nil, err
	}

	var engine *xorm.Engine
	if len(setting.Database.Schema) > 0 {
		EnsureSchemaDriverRegistered()
		engine, err = xorm.NewEngine("postgresschema", connStr)
	} else {
		engine, err = xorm.NewEngine("postgres", connStr)
	}
	if err != nil {
		return nil, err
	}

	engine.SetSchema(setting.Database.Schema)
	return engine, nil
}

func configureEngine(xe *xorm.Engine) {
	xe.SetMapper(names.GonicMapper{})
	xe.SetLogger(db.NewXORMLogger(setting.Database.LogSQL))
	xe.ShowSQL(setting.Database.LogSQL)
	xe.SetMaxOpenConns(setting.Database.MaxOpenConns)
	xe.SetMaxIdleConns(setting.Database.MaxIdleConns)
	xe.SetConnMaxLifetime(setting.Database.ConnMaxLifetime)

	if setting.Database.SlowQueryThreshold > 0 {
		xe.AddHook(&db.EngineHook{
			Threshold: setting.Database.SlowQueryThreshold,
			Logger:    log.GetLogger("xorm"),
		})
	}
}

// Init implements metadatastore.Backend.
func (*Backend) Init(ctx context.Context) error {
	xe, err := newXORMEngine()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	configureEngine(xe)
	db.SetDefaultEngine(ctx, xe)
	return nil
}

// Unset implements metadatastore.Backend.
func (*Backend) Unset() {
	db.UnsetDefaultEngine()
}

// InitWithMigration implements metadatastore.Backend.
func (*Backend) InitWithMigration(ctx context.Context, migrateFunc func(context.Context, *xorm.Engine) error) error {
	xe, err := newXORMEngine()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	configureEngine(xe)
	db.SetDefaultEngine(ctx, xe)

	if err := xe.Ping(); err != nil {
		return err
	}

	db.PreprocessDatabaseCollation(xe)

	if err := migrateFunc(ctx, xe); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if err := db.SyncAllTables(); err != nil {
		return fmt.Errorf("sync database struct error: %w", err)
	}

	if err := db.RunRegisteredInitFuncs(); err != nil {
		return err
	}

	return nil
}
