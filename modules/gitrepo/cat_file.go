// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
)

func NewBatch(ctx context.Context, repo Repository) (git.CatFileBatchCloser, error) {
	if isRemoteBackendEnabled() {
		return &remoteCatFileBatch{ctx: ctx, repo: repo}, nil
	}
	return git.NewBatch(ctx, repoPath(repo))
}

type remoteCatFileBatch struct {
	ctx  context.Context
	repo Repository
}

func (b *remoteCatFileBatch) Close() {}

func (b *remoteCatFileBatch) QueryInfo(obj string) (*git.CatFileObject, error) {
	typOut, _, err := RunCmdString(b.ctx, b.repo,
		gitcmd.NewCommand("cat-file", "-t").AddDynamicArguments(obj))
	if err != nil {
		return nil, err
	}
	sizeOut, _, err := RunCmdString(b.ctx, b.repo,
		gitcmd.NewCommand("cat-file", "-s").AddDynamicArguments(obj))
	if err != nil {
		return nil, err
	}
	size, convErr := strconv.ParseInt(strings.TrimSpace(sizeOut), 10, 64)
	if convErr != nil {
		return nil, fmt.Errorf("parse object size from %q: %w", sizeOut, convErr)
	}
	return &git.CatFileObject{
		ID:   obj,
		Type: strings.TrimSpace(typOut),
		Size: size,
	}, nil
}

func (b *remoteCatFileBatch) QueryContent(obj string) (*git.CatFileObject, git.BufferedReader, error) {
	info, err := b.QueryInfo(obj)
	if err != nil {
		return nil, nil, err
	}
	stdout, _, runErr := RunCmdBytes(b.ctx, b.repo, gitcmd.NewCommand("cat-file", "-p").AddDynamicArguments(obj))
	if runErr != nil {
		return nil, nil, runErr
	}
	return info, bufio.NewReader(bytes.NewReader(stdout)), nil
}
