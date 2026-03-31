// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"strings"

	"github.com/gitjet-ru/core-scm/modules/setting"

	"xorm.io/builder"
)

// BuildCaseInsensitiveLike returns a case-insensitive LIKE condition for the given key and value.
func BuildCaseInsensitiveLike(key, value string) builder.Cond {
	return builder.Like{"LOWER(" + key + ")", strings.ToLower(value)}
}

// BuildCaseInsensitiveIn returns a condition to check if the given value is in the given values case-insensitively.
func BuildCaseInsensitiveIn(key string, values []string) builder.Cond {
	incaseValues := make([]string, len(values))
	for i, value := range values {
		incaseValues[i] = strings.ToLower(value)
	}
	return builder.In("LOWER("+key+")", incaseValues)
}

// BuilderDialect returns the xorm.Builder dialect of the engine (PostgreSQL only).
func BuilderDialect() string {
	if setting.Database.Type.IsPostgreSQL() {
		return builder.POSTGRES
	}
	return builder.POSTGRES
}
