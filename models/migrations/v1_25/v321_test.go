// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/migrations/base"
)

func Test_UseLongTextInSomeColumnsAndFixBugs(t *testing.T) {
	x, deferrable := base.PrepareTestEnv(t, 0)
	defer deferrable()
	_ = UseLongTextInSomeColumnsAndFixBugs(x)
}
