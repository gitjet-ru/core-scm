// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"

	_ "github.com/gitjet-ru/core-scm/models" // register models
	_ "github.com/gitjet-ru/core-scm/models/actions"
	_ "github.com/gitjet-ru/core-scm/models/activities"
	_ "github.com/gitjet-ru/core-scm/models/system" // register models of system
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
