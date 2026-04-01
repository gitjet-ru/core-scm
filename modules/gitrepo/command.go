// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

func RunCmd(ctx context.Context, repo Repository, cmd *gitcmd.Command) error {
	if isRemoteBackendEnabled() {
		stdout, stderr, execErr, transportErr := runRemoteCommand(ctx, repo, cmd)
		_ = stdout
		if transportErr != nil {
			return transportErr
		}
		if execErr != "" {
			return errors.New(execErr + " - " + stderr)
		}
		return nil
	}
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().Run(ctx)
}

func RunCmdString(ctx context.Context, repo Repository, cmd *gitcmd.Command) (string, string, gitcmd.RunStdError) {
	if isRemoteBackendEnabled() {
		stdout, stderr, execErr, transportErr := runRemoteCommand(ctx, repo, cmd)
		if transportErr != nil {
			return "", "", &remoteRunStdErr{err: transportErr}
		}
		if execErr != "" {
			return stdout, stderr, &remoteRunStdErr{err: errors.New(execErr), stderr: stderr}
		}
		return stdout, stderr, nil
	}
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().RunStdString(ctx)
}

func RunCmdBytes(ctx context.Context, repo Repository, cmd *gitcmd.Command) ([]byte, []byte, gitcmd.RunStdError) {
	if isRemoteBackendEnabled() {
		stdout, stderr, execErr, transportErr := runRemoteCommandBytes(ctx, repo, cmd)
		if transportErr != nil {
			return nil, nil, &remoteRunStdErr{err: transportErr}
		}
		if execErr != "" {
			return stdout, stderr, &remoteRunStdErr{err: errors.New(execErr), stderr: string(stderr)}
		}
		return stdout, stderr, nil
	}
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().RunStdBytes(ctx)
}

func RunCmdWithStderr(ctx context.Context, repo Repository, cmd *gitcmd.Command) gitcmd.RunStdError {
	if isRemoteBackendEnabled() {
		spec := cmd.WithParentCallerInfo().ExportRemoteSpec()
		if spec.HasCustomIO || spec.HasPipeline || spec.HasPreErrors {
			return &remoteRunStdErr{err: fmt.Errorf("remote backend does not support custom IO/pipeline git command: %v", spec.Args)}
		}
		_, stderr, execErr, transportErr := runRemoteCommand(ctx, repo, cmd)
		if transportErr != nil {
			return &remoteRunStdErr{err: transportErr}
		}
		if execErr != "" {
			return &remoteRunStdErr{err: errors.New(execErr), stderr: stderr}
		}
		return nil
	}
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().RunWithStderr(ctx)
}

type remoteRunStdErr struct {
	err    error
	stderr string
}

func (e *remoteRunStdErr) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return e.err.Error() + " - " + e.stderr
}

func (e *remoteRunStdErr) Unwrap() error  { return e.err }
func (e *remoteRunStdErr) Stderr() string { return e.stderr }

func runRemoteCommand(ctx context.Context, repo Repository, cmd *gitcmd.Command) (string, string, string, error) {
	stdout, stderr, execErr, err := runRemoteCommandBytes(ctx, repo, cmd)
	return string(stdout), string(stderr), execErr, err
}

func runRemoteCommandBytes(ctx context.Context, repo Repository, cmd *gitcmd.Command) ([]byte, []byte, string, error) {
	spec := cmd.WithParentCallerInfo().ExportRemoteSpec()
	if spec.HasCustomIO || spec.HasPipeline || spec.HasPreErrors {
		return nil, nil, "", fmt.Errorf("remote backend does not support custom IO/pipeline git command: %v", spec.Args)
	}
	client, err := getRemoteClient()
	if err != nil {
		return nil, nil, "", err
	}
	var resp *gitstoragev1.RunGitCommandResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.RunGitCommand(cctx, &gitstoragev1.RunGitCommandRequest{
			RepoRelativePath: repo.RelativePath(),
			Args:             spec.Args,
			Env:              spec.Env,
			Stdin:            spec.Stdin,
		})
		return reqErr
	})
	if err != nil {
		return nil, nil, "", err
	}
	if resp.GetExecError() == "" && isMutatingGitArgs(spec.Args) {
		invalidateRemoteMirror(repo.RelativePath())
	}
	return resp.GetStdout(), resp.GetStderr(), resp.GetExecError(), nil
}

func isMutatingGitArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "update-ref", "symbolic-ref", "branch", "tag", "commit-tree", "write-tree", "reset",
		"revert", "cherry-pick", "merge", "rebase", "checkout", "switch", "fetch", "pull", "push",
		"fast-import",
		"gc", "prune", "pack-refs", "repack":
		return true
	default:
		return false
	}
}
