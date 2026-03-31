// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"

	_ "github.com/gitjet-ru/core-scm/models"
	_ "github.com/gitjet-ru/core-scm/models/actions"
	_ "github.com/gitjet-ru/core-scm/models/activities"
	_ "github.com/gitjet-ru/core-scm/models/user"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
