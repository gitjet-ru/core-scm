// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"time"

	git_model "github.com/gitjet-ru/core-scm/models/git"
	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/setting"
)

// GarbageCollectLFSMetaObjectsOptions provides options for GarbageCollectLFSMetaObjects function
type GarbageCollectLFSMetaObjectsOptions struct {
	LogDetail                func(format string, v ...any)
	AutoFix                  bool
	OlderThan                time.Time
	UpdatedLessRecentlyThan  time.Time
	NumberToCheckPerRepo     int64
	ProportionToCheckPerRepo float64
}

// GarbageCollectLFSMetaObjects garbage collects LFS objects for all repositories
func GarbageCollectLFSMetaObjects(ctx context.Context, opts GarbageCollectLFSMetaObjectsOptions) error {
	log.Trace("Doing: GarbageCollectLFSMetaObjects")
	defer log.Trace("Finished: GarbageCollectLFSMetaObjects")

	if opts.LogDetail == nil {
		opts.LogDetail = log.Debug
	}

	if !setting.LFS.StartServer {
		opts.LogDetail("LFS support is disabled")
		return nil
	}

	return git_model.IterateRepositoryIDsWithLFSMetaObjects(ctx, func(ctx context.Context, repoID, count int64) error {
		repo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil {
			return err
		}

		if newMinimum := int64(float64(count) * opts.ProportionToCheckPerRepo); newMinimum > opts.NumberToCheckPerRepo && opts.NumberToCheckPerRepo != 0 {
			opts.NumberToCheckPerRepo = newMinimum
		}
		return GarbageCollectLFSMetaObjectsForRepo(ctx, repo, opts)
	})
}

// GarbageCollectLFSMetaObjectsForRepo garbage collects LFS objects for a specific repository
func GarbageCollectLFSMetaObjectsForRepo(ctx context.Context, repo *repo_model.Repository, opts GarbageCollectLFSMetaObjectsOptions) error {
	opts.LogDetail("Checking %-v", repo)
	opts.LogDetail("Skipping LFS GC in mirrorless hard-cut for %-v", repo)
	return nil
}
