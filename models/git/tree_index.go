// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"

	"github.com/gitjet-ru/core-scm/models/db"
	"github.com/gitjet-ru/core-scm/modules/base"
	"xorm.io/builder"
)

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
	EntryType string `xorm:"VARCHAR(16) index"` // dir,file,symlink,submodule
	Size      int64
}

func init() {
	db.RegisterModel(new(BranchTreeIndex))
	db.RegisterModel(new(BranchTreeEntry))
}

// ErrBranchTreeIndexNotReady is returned when a branch index row exists but is not yet usable for reads.
var ErrBranchTreeIndexNotReady = errors.New("branch tree index not ready")

// BranchTreeEntrySortsBeforeBlobs matches git.TreeEntry listing: trees and submodules first (see Entries.CustomSort).
// Prefer git EntryMode from ls-tree; EntryType alone can be stale or inconsistent across indexer versions.
func BranchTreeEntrySortsBeforeBlobs(e *BranchTreeEntry) bool {
	m := strings.TrimSpace(e.EntryMode)
	if len(m) >= 2 {
		// modes: tree 04*, blob 10*, symlink 12*, commit/submodule 16*
		if m[0] == '0' && m[1] == '4' {
			return true
		}
		if m[0] == '1' && m[1] == '6' {
			return true
		}
		return false
	}
	return e.EntryType == "dir" || e.EntryType == "submodule"
}

// SortBranchTreeEntriesForListing applies the same ordering as git.Entries.CustomSort(base.NaturalSortCompare).
func SortBranchTreeEntriesForListing(entries []*BranchTreeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		ei, ej := entries[i], entries[j]
		di, dj := BranchTreeEntrySortsBeforeBlobs(ei), BranchTreeEntrySortsBeforeBlobs(ej)
		if di != dj {
			return di
		}
		return base.NaturalSortCompare(ei.EntryName, ej.EntryName) < 0
	})
}

func UpsertBranchTreeIndex(ctx context.Context, idx *BranchTreeIndex) error {
	has, err := db.GetEngine(ctx).Where("repo_id=? AND branch=?", idx.RepoID, idx.Branch).Exist(&BranchTreeIndex{})
	if err != nil {
		return err
	}
	if !has {
		_, err = db.GetEngine(ctx).Insert(idx)
		return err
	}
	_, err = db.GetEngine(ctx).
		Where("repo_id=? AND branch=?", idx.RepoID, idx.Branch).
		Cols("head_commit_id", "version", "state", "indexed_unix").
		Update(idx)
	return err
}

func ReplaceBranchTreeEntries(ctx context.Context, repoID int64, branch string, version int64, entries []*BranchTreeEntry) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("repo_id=? AND branch=? AND version<>?", repoID, branch, version).Delete(&BranchTreeEntry{}); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Where("repo_id=? AND branch=? AND version=?", repoID, branch, version).Delete(&BranchTreeEntry{}); err != nil {
			return err
		}
		for _, e := range entries {
			if _, err := db.GetEngine(ctx).Insert(e); err != nil {
				return err
			}
		}
		return nil
	})
}

func DeleteBranchTreeIndex(ctx context.Context, repoID int64, branch string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("repo_id=? AND branch=?", repoID, branch).Delete(&BranchTreeIndex{}); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Where("repo_id=? AND branch=?", repoID, branch).Delete(&BranchTreeEntry{}); err != nil {
			return err
		}
		return nil
	})
}

func GetBranchTreeIndex(ctx context.Context, repoID int64, branch string) (*BranchTreeIndex, error) {
	var idx BranchTreeIndex
	has, err := db.GetEngine(ctx).Where("repo_id=? AND branch=?", repoID, branch).Get(&idx)
	if err != nil || !has {
		return nil, err
	}
	return &idx, nil
}

func FindBranchTreeEntries(ctx context.Context, repoID int64, branch, dirPath string) ([]*BranchTreeEntry, error) {
	idx, err := GetBranchTreeIndex(ctx, repoID, branch)
	if err != nil || idx == nil {
		return nil, err
	}
	if idx.State != "ready" {
		return nil, ErrBranchTreeIndexNotReady
	}
	entries := make([]*BranchTreeEntry, 0, 64)
	err = db.GetEngine(ctx).
		Where(builder.Eq{
			"repo_id":  repoID,
			"branch":   branch,
			"version":  idx.Version,
			"dir_path": strings.Trim(dirPath, "/"),
		}).
		// Order only for stable scans; web/API apply git-like sort in Go (dirs first + natural name).
		OrderBy("entry_name ASC").
		Find(&entries)
	return entries, err
}

func FindBranchTreeEntryByPath(ctx context.Context, repoID int64, branch, fullPath string) (*BranchTreeEntry, error) {
	idx, err := GetBranchTreeIndex(ctx, repoID, branch)
	if err != nil || idx == nil {
		return nil, err
	}
	if idx.State != "ready" {
		return nil, ErrBranchTreeIndexNotReady
	}
	clean := strings.Trim(fullPath, "/")
	dirPath, name := path.Split(clean)
	dirPath = strings.TrimSuffix(strings.Trim(dirPath, "/"), "/")
	var entry BranchTreeEntry
	has, err := db.GetEngine(ctx).Where(builder.Eq{
		"repo_id":    repoID,
		"branch":     branch,
		"version":    idx.Version,
		"dir_path":   dirPath,
		"entry_name": name,
	}).Get(&entry)
	if err != nil || !has {
		return nil, err
	}
	return &entry, nil
}
