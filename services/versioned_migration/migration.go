// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package versioned_migration

import (
	"context"

	"github.com/gitjet-ru/core-scm/models/migrations"
	"github.com/gitjet-ru/core-scm/modules/globallock"

	"xorm.io/xorm"
)

func Migrate(ctx context.Context, x *xorm.Engine) error {
	// only one instance can do the migration at the same time if there are multiple instances
	release, err := globallock.Lock(ctx, "gitea_versioned_migration")
	if err != nil {
		return err
	}
	defer release()

	return migrations.Migrate(ctx, x)
}
