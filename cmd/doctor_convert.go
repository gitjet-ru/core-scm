// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/setting"

	"github.com/urfave/cli/v3"
)

func newDoctorConvertCommand() *cli.Command {
	return &cli.Command{
		Name:        "convert",
		Usage:       "Convert the database",
		Description: "No-op for GitJet (PostgreSQL only); retained for CLI compatibility",
		Action:      runDoctorConvert,
	}
}

func runDoctorConvert(ctx context.Context, cmd *cli.Command) error {
	if err := initDB(ctx); err != nil {
		return err
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.Log.RootPath)
	log.Info("Configuration file: %s", setting.CustomConf)

	log.Info("GitJet uses PostgreSQL only; MySQL/MSSQL charset conversion is not applicable.")
	fmt.Println("No conversion performed.")
	return nil
}
