// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

func init() {
	gonicNames := []string{"SSL", "UID"}
	for _, name := range gonicNames {
		names.LintGonicMapper[name] = true
	}
}

// SetDefaultEngine sets the default engine for db
func SetDefaultEngine(ctx context.Context, eng *xorm.Engine) {
	xormEngine = eng
	xormEngine.SetDefaultContext(ctx)
}

// UnsetDefaultEngine closes and unsets the default engine
// We hope the SetDefaultEngine and UnsetDefaultEngine can be paired, but it's impossible now,
// there are many calls to InitEngine -> SetDefaultEngine directly to overwrite the `xormEngine` and `xormContext` without close
// Global database engine related functions are all racy and there is no graceful close right now.
func UnsetDefaultEngine() {
	if xormEngine != nil {
		_ = xormEngine.Close()
		xormEngine = nil
	}
}

// RunRegisteredInitFuncs runs all model init callbacks registered with RegisterModel.
func RunRegisteredInitFuncs() error {
	for _, initFunc := range registeredInitFuncs {
		if err := initFunc(); err != nil {
			return fmt.Errorf("initFunc failed: %w", err)
		}
	}
	return nil
}
