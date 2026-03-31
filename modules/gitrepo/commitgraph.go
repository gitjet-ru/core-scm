// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
)

func WriteCommitGraph(ctx context.Context, repo Repository) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("commit-graph", "write", "--reachable"))
}
