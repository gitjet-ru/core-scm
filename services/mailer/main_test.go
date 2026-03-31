// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"

	_ "github.com/gitjet-ru/core-scm/models"
	_ "github.com/gitjet-ru/core-scm/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}
