// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"github.com/gitjet-ru/core-scm/models/db"
	"github.com/gitjet-ru/core-scm/services/context"
	"github.com/gitjet-ru/core-scm/services/convert"
)

// GetListOptions returns list options using the page and limit parameters
func GetListOptions(ctx *context.APIContext) db.ListOptions {
	return db.ListOptions{
		Page:     max(ctx.FormInt("page"), 1),
		PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
	}
}
