// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"testing"

	"github.com/gitjet-ru/core-scm/models/unittest"

	"github.com/stretchr/testify/assert"
)

type TestCollationTbl struct {
	ID  int64
	Txt string `xorm:"VARCHAR(10) UNIQUE"`
}

func TestDatabaseCollation(t *testing.T) {
	x := unittest.GetXORMEngine()

	// all created tables should use case-sensitive collation by default
	_, _ = x.Exec("DROP TABLE IF EXISTS test_collation_tbl")
	err := x.Sync(&TestCollationTbl{})
	assert.NoError(t, err)
	_, _ = x.Exec("INSERT INTO test_collation_tbl (txt) VALUES ('main')")
	_, _ = x.Exec("INSERT INTO test_collation_tbl (txt) VALUES ('Main')") // case-sensitive, so it inserts a new row
	_, _ = x.Exec("INSERT INTO test_collation_tbl (txt) VALUES ('main')") // duplicate, so it doesn't insert
	cnt, err := x.Count(&TestCollationTbl{})
	assert.NoError(t, err)
	assert.EqualValues(t, 2, cnt)
	_, _ = x.Exec("DROP TABLE IF EXISTS test_collation_tbl")

}
