// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"github.com/gitjet-ru/core-scm/modules/git"
	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

// CloneExternalRepo clones an external repository to the managed repository.
func CloneExternalRepo(ctx context.Context, fromRemoteURL string, toRepo Repository, opts git.CloneRepoOptions) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.CloneExternalToRepository(cctx, &gitstoragev1.CloneExternalToRepositoryRequest{
				FromRemoteUrl:      fromRemoteURL,
				ToRepoRelativePath: toRepo.RelativePath(),
				Mirror:             opts.Mirror,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(toRepo.RelativePath())
		}
		return err
	}
	return git.Clone(ctx, fromRemoteURL, repoPath(toRepo), opts)
}

// CloneRepoToLocal clones a managed repository to a local path.
func CloneRepoToLocal(ctx context.Context, fromRepo Repository, toLocalPath string, opts git.CloneRepoOptions) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		return callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.CloneRepositoryToLocal(cctx, &gitstoragev1.CloneRepositoryToLocalRequest{
				FromRepoRelativePath: fromRepo.RelativePath(),
				ToLocalPath:          toLocalPath,
				Mirror:               opts.Mirror,
			})
			return err
		})
	}
	return git.Clone(ctx, repoPath(fromRepo), toLocalPath, opts)
}

func Clone(ctx context.Context, fromRepo, toRepo Repository, opts git.CloneRepoOptions) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.CloneRepository(cctx, &gitstoragev1.CloneRepositoryRequest{
				FromRepoRelativePath: fromRepo.RelativePath(),
				ToRepoRelativePath:   toRepo.RelativePath(),
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(toRepo.RelativePath())
		}
		return err
	}
	return git.Clone(ctx, repoPath(fromRepo), repoPath(toRepo), opts)
}
