// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"
	"github.com/gitjet-ru/core-scm/modules/setting"
	webhook_service "github.com/gitjet-ru/core-scm/services/webhook"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			setting.LoadQueueSettings()
			return webhook_service.Init()
		},
	})
}
