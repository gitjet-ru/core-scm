// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

// MergeBase checks and returns merge base of two commits.
func MergeBase(ctx context.Context, repo Repository, baseCommitID, headCommitID string) (string, error) {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return "", err
		}
		var mergeBase string
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			resp, err := client.MergeBase(cctx, &gitstoragev1.MergeBaseRequest{
				RepoRelativePath: repo.RelativePath(),
				BaseCommitId:     baseCommitID,
				HeadCommitId:     headCommitID,
			})
			if err != nil {
				return err
			}
			mergeBase = resp.GetMergeBaseCommitId()
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("get merge-base of %s and %s failed: %w", baseCommitID, headCommitID, err)
		}
		return strings.TrimSpace(mergeBase), nil
	}
	mergeBase, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("merge-base").
		AddDashesAndList(baseCommitID, headCommitID))
	if err != nil {
		return "", fmt.Errorf("get merge-base of %s and %s failed: %w", baseCommitID, headCommitID, err)
	}
	return strings.TrimSpace(mergeBase), nil
}
