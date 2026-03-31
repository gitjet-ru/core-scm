// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"github.com/gitjet-ru/core-scm/modules/git"
	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

// PushToExternal pushes a managed repository to an external remote.
func PushToExternal(ctx context.Context, repo Repository, opts git.PushOptions) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		return callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.PushRepositoryToExternal(cctx, &gitstoragev1.PushRepositoryToExternalRequest{
				FromRepoRelativePath: repo.RelativePath(),
				RemoteUrl:            opts.Remote,
				LocalRefName:         opts.LocalRefName,
				Branch:               opts.Branch,
				Force:                opts.Force,
				ForceWithLease:       opts.ForceWithLease,
				Mirror:               opts.Mirror,
			})
			return err
		})
	}
	return git.Push(ctx, repoPath(repo), opts)
}

// Push pushes from one managed repository to another managed repository.
func Push(ctx context.Context, fromRepo, toRepo Repository, opts git.PushOptions) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.PushRepository(cctx, &gitstoragev1.PushRepositoryRequest{
				FromRepoRelativePath: fromRepo.RelativePath(),
				ToRepoRelativePath:   toRepo.RelativePath(),
				LocalRefName:         opts.LocalRefName,
				Branch:               opts.Branch,
				Force:                opts.Force,
				ForceWithLease:       opts.ForceWithLease,
				Mirror:               opts.Mirror,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(toRepo.RelativePath())
		}
		return err
	}
	opts.Remote = repoPath(toRepo)
	return git.Push(ctx, repoPath(fromRepo), opts)
}

// PushFromLocal pushes from a local path to a managed repository.
func PushFromLocal(ctx context.Context, fromLocalPath string, toRepo Repository, opts git.PushOptions) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.PushLocalToRepository(cctx, &gitstoragev1.PushLocalToRepositoryRequest{
				FromLocalPath:      fromLocalPath,
				ToRepoRelativePath: toRepo.RelativePath(),
				LocalRefName:       opts.LocalRefName,
				Branch:             opts.Branch,
				Force:              opts.Force,
				ForceWithLease:     opts.ForceWithLease,
				Mirror:             opts.Mirror,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(toRepo.RelativePath())
		}
		return err
	}
	opts.Remote = repoPath(toRepo)
	return git.Push(ctx, fromLocalPath, opts)
}
