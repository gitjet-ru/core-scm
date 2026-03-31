// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"

	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/metadatastore"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/services/versioned_migration"

	"github.com/urfave/cli/v3"
)

func newMigrateCommand() *cli.Command {
	return &cli.Command{
		Name:        "migrate",
		Usage:       "Migrate the database",
		Description: `This is a command for migrating the database, so that you can run "gitea admin create user" before starting the server.`,
		Action:      runMigrate,
	}
}

func runMigrate(ctx context.Context, c *cli.Command) error {
	setting.MustInstalled()
	setting.LoadDBSetting()
	setting.InitSQLLoggersForCli(log.INFO)

	if setting.Database.Type == "" {
		log.Fatal(`Database settings are missing from the configuration file: %q.
Ensure you are running in the correct environment or set the correct configuration file with -c.
If this is the intended configuration file complete the [database] section.`, setting.CustomConf)
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.Log.RootPath)
	log.Info("Configuration file: %s", setting.CustomConf)

	if err := metadatastore.Default().InitWithMigration(context.Background(), versioned_migration.Migrate); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	return nil
}
