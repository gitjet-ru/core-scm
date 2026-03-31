// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

// GetBranchesByPath returns a branch by its path
// if limit = 0 it will not limit
func GetBranchesByPath(ctx context.Context, repo Repository, skip, limit int) ([]string, int, error) {
	stdout, _, err := RunCmdString(ctx, repo,
		gitcmd.NewCommand("for-each-ref", "--format=%(refname:short)", git.BranchPrefix))
	if err != nil {
		return nil, 0, err
	}
	all := make([]string, 0, 64)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		all = append(all, line)
	}
	total := len(all)
	if skip >= total {
		return []string{}, total, nil
	}
	from := skip
	to := total
	if limit > 0 && from+limit < to {
		to = from + limit
	}
	return all[from:to], total, nil
}

func GetBranchCommitID(ctx context.Context, repo Repository, branch string) (string, error) {
	stdout, _, err := RunCmdString(ctx, repo,
		gitcmd.NewCommand("rev-parse").AddDynamicArguments(git.BranchPrefix+branch))
	if err != nil {
		return "", fmt.Errorf("get branch commit id for %s failed: %w", branch, err)
	}
	return strings.TrimSpace(stdout), nil
}

// SetDefaultBranch sets default branch of repository.
func SetDefaultBranch(ctx context.Context, repo Repository, name string) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.SetHead(cctx, &gitstoragev1.SetHeadRequest{
				RepoRelativePath: repo.RelativePath(),
				BranchName:       name,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return err
	}
	_, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("symbolic-ref", "HEAD").
		AddDynamicArguments(git.BranchPrefix+name))
	return err
}

// GetDefaultBranch gets default branch of repository.
func GetDefaultBranch(ctx context.Context, repo Repository) (string, error) {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return "", err
		}
		var branch string
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			resp, err := client.GetDefaultBranch(cctx, &gitstoragev1.GetDefaultBranchRequest{
				RepoRelativePath: repo.RelativePath(),
			})
			if err != nil {
				return err
			}
			branch = resp.GetBranchName()
			return nil
		})
		return branch, err
	}
	stdout, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("symbolic-ref", "HEAD"))
	if err != nil {
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if !strings.HasPrefix(stdout, git.BranchPrefix) {
		return "", errors.New("the HEAD is not a branch: " + stdout)
	}
	return strings.TrimPrefix(stdout, git.BranchPrefix), nil
}

// IsReferenceExist returns true if given reference exists in the repository.
func IsReferenceExist(ctx context.Context, repo Repository, name string) bool {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return false
		}
		exists := false
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			resp, err := client.IsReferenceExist(cctx, &gitstoragev1.IsReferenceExistRequest{
				RepoRelativePath: repo.RelativePath(),
				ReferenceName:    name,
			})
			if err != nil {
				return err
			}
			exists = resp.GetExists()
			return nil
		})
		return err == nil && exists
	}
	_, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("show-ref", "--verify").AddDashesAndList(name))
	return err == nil
}

// IsBranchExist returns true if given branch exists in the repository.
func IsBranchExist(ctx context.Context, repo Repository, name string) bool {
	return IsReferenceExist(ctx, repo, git.BranchPrefix+name)
}

// DeleteBranch delete a branch by name on repository.
func DeleteBranch(ctx context.Context, repo Repository, name string, force bool) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.DeleteBranch(cctx, &gitstoragev1.DeleteBranchRequest{
				RepoRelativePath: repo.RelativePath(),
				BranchName:       name,
				Force:            force,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return err
	}
	cmd := gitcmd.NewCommand("branch")

	if force {
		cmd.AddArguments("-D")
	} else {
		cmd.AddArguments("-d")
	}

	cmd.AddDashesAndList(name)
	_, _, err := RunCmdString(ctx, repo, cmd)
	return err
}

// CreateBranch create a new branch
func CreateBranch(ctx context.Context, repo Repository, branch, oldbranchOrCommit string) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.CreateBranch(cctx, &gitstoragev1.CreateBranchRequest{
				RepoRelativePath: repo.RelativePath(),
				BranchName:       branch,
				StartPoint:       oldbranchOrCommit,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return err
	}
	cmd := gitcmd.NewCommand("branch")
	cmd.AddDashesAndList(branch, oldbranchOrCommit)

	_, _, err := RunCmdString(ctx, repo, cmd)
	return err
}

// RenameBranch rename a branch
func RenameBranch(ctx context.Context, repo Repository, from, to string) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.RenameBranch(cctx, &gitstoragev1.RenameBranchRequest{
				RepoRelativePath: repo.RelativePath(),
				FromBranch:       from,
				ToBranch:         to,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return err
	}
	_, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("branch", "-m").AddDynamicArguments(from, to))
	return err
}
