// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"

	_ "github.com/gitjet-ru/core-scm/models/actions"
	_ "github.com/gitjet-ru/core-scm/models/activities"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
