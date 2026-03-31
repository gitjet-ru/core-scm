// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package gitrepo

import (
	"context"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
)

// WalkReferences walks all the references from the repository
func WalkReferences(ctx context.Context, repo Repository, walkfn func(sha1, refname string) error) (int, error) {
	stdout, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("show-ref"))
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		if err := walkfn(parts[0], parts[1]); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
