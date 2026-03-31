// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"

	_ "github.com/gitjet-ru/core-scm/models" // register table model
	_ "github.com/gitjet-ru/core-scm/models/actions"
	_ "github.com/gitjet-ru/core-scm/models/activities"
	_ "github.com/gitjet-ru/core-scm/models/perm/access" // register table model
	_ "github.com/gitjet-ru/core-scm/models/repo"        // register table model
	_ "github.com/gitjet-ru/core-scm/models/user"        // register table model
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
