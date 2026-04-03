// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

func AddBranchTreeIndexTables(x *xorm.Engine) error {
	type BranchTreeIndex struct {
		ID           int64  `xorm:"pk autoincr"`
		RepoID       int64  `xorm:"index UNIQUE(s)"`
		Branch       string `xorm:"UNIQUE(s)"`
		HeadCommitID string `xorm:"VARCHAR(64)"`
		Version      int64  `xorm:"index"`
		State        string `xorm:"VARCHAR(16) index"`
		IndexedUnix  int64  `xorm:"index"`
		CreatedUnix  int64  `xorm:"created"`
		UpdatedUnix  int64  `xorm:"updated"`
	}
	type BranchTreeEntry struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"index UNIQUE(s)"`
		Branch    string `xorm:"UNIQUE(s)"`
		Version   int64  `xorm:"UNIQUE(s)"`
		DirPath   string `xorm:"UNIQUE(s) VARCHAR(1024)"`
		EntryName string `xorm:"UNIQUE(s) VARCHAR(512)"`
		EntryMode string `xorm:"VARCHAR(16)"`
		ObjectID  string `xorm:"VARCHAR(64) index"`
		EntryType string `xorm:"VARCHAR(16) index"`
		Size      int64
	}
	if err := x.Sync(new(BranchTreeIndex)); err != nil {
		return err
	}
	return x.Sync(new(BranchTreeEntry))
}
