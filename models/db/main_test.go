// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"

	_ "github.com/gitjet-ru/core-scm/models"
	_ "github.com/gitjet-ru/core-scm/models/repo"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
