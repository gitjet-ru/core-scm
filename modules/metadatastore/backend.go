// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package metadatastore defines the boundary between application code and relational
// metadata persistence (PostgreSQL via xorm). Object/blob storage lives in modules/storage
// and is intentionally separate.
package metadatastore

import (
	"context"

	"xorm.io/xorm"
)

// Backend manages the lifecycle of the metadata database engine (connect, migrate, sync).
// Implementations must register the global xorm engine via models/db.SetDefaultEngine.
type Backend interface {
	// Init opens a connection, configures the engine, and sets it as the default db engine.
	Init(ctx context.Context) error
	// InitWithMigration runs Init, then ping, collation preprocessing, migrateFunc, schema sync, and model init hooks.
	InitWithMigration(ctx context.Context, migrateFunc func(context.Context, *xorm.Engine) error) error
	// Unset closes and clears the default engine.
	Unset()
}
