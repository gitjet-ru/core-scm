// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"os"
	"testing"

	"github.com/gitjet-ru/core-scm/modules/markup"
	"github.com/gitjet-ru/core-scm/modules/setting"
)

func TestMain(m *testing.M) {
	setting.IsInTesting = true
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	setting.Markdown.FileNamePatterns = []string{"*.md"}
	markup.RefreshFileNamePatterns()
	os.Exit(m.Run())
}
