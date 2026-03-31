// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"
	"github.com/gitjet-ru/core-scm/modules/hostmatcher"
	"github.com/gitjet-ru/core-scm/modules/setting"

	_ "github.com/gitjet-ru/core-scm/models"
	_ "github.com/gitjet-ru/core-scm/models/actions"
)

func TestMain(m *testing.M) {
	// for tests, allow only loopback IPs
	setting.Webhook.AllowedHostList = hostmatcher.MatchBuiltinLoopback
	unittest.MainTest(m, &unittest.TestOptions{
		SetUp: func() error {
			setting.LoadQueueSettings()
			return Init()
		},
	})
}
