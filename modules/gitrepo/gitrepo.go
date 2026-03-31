// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	"github.com/gitjet-ru/core-scm/modules/reqctx"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/util"
	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

// Repository represents a git repository which stored in a disk
type Repository interface {
	RelativePath() string // We don't assume how the directory structure of the repository is, so we only need the relative path
}

// repoPath resolves the Repository.RelativePath (which is a unix-style path like "username/reponame.git")
// to a local filesystem path according to setting.RepoRootPath
var repoPath = func(repo Repository) string {
	return filepath.Join(setting.RepoRootPath, filepath.FromSlash(repo.RelativePath()))
}

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	if isRemoteBackendEnabled() {
		localPath, err := ensureRemoteMirror(ctx, repo)
		if err != nil {
			return nil, err
		}
		return git.OpenRepository(ctx, localPath)
	}
	return git.OpenRepository(ctx, repoPath(repo))
}

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	repoPath string
}

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
// The caller must call "defer gitRepo.Close()"
func RepositoryFromContextOrOpen(ctx context.Context, repo Repository) (*git.Repository, io.Closer, error) {
	reqCtx := reqctx.FromContext(ctx)
	if reqCtx != nil {
		gitRepo, err := RepositoryFromRequestContextOrOpen(reqCtx, repo)
		return gitRepo, util.NopCloser{}, err
	}
	gitRepo, err := OpenRepository(ctx, repo)
	return gitRepo, gitRepo, err
}

// RepositoryFromRequestContextOrOpen opens the repository at the given relative path in the provided request context.
// Caller shouldn't close the git repo manually, the git repo will be automatically closed when the request context is done.
func RepositoryFromRequestContextOrOpen(ctx reqctx.RequestContext, repo Repository) (*git.Repository, error) {
	openPath := repoPath(repo)
	if isRemoteBackendEnabled() {
		localPath, err := ensureRemoteMirror(ctx, repo)
		if err != nil {
			return nil, err
		}
		openPath = localPath
	}
	ck := contextKey{repoPath: openPath}
	if gitRepo, ok := ctx.Value(ck).(*git.Repository); ok {
		return gitRepo, nil
	}
	gitRepo, err := git.OpenRepository(ctx, ck.repoPath)
	if err != nil {
		return nil, err
	}
	ctx.AddCloser(gitRepo)
	ctx.SetContextValue(ck, gitRepo)
	return gitRepo, nil
}

// IsRepositoryExist returns true if the repository directory exists in the disk
func IsRepositoryExist(ctx context.Context, repo Repository) (bool, error) {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return false, err
		}
		exists := false
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			resp, err := client.RepositoryExists(cctx, &gitstoragev1.RepositoryExistsRequest{
				RepoRelativePath: repo.RelativePath(),
			})
			if err != nil {
				return err
			}
			exists = resp.GetExists()
			return nil
		})
		return exists, err
	}
	return util.IsExist(repoPath(repo))
}

// DeleteRepository deletes the repository directory from the disk, it will return
// nil if the repository does not exist.
func DeleteRepository(ctx context.Context, repo Repository) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.DeleteRepository(cctx, &gitstoragev1.DeleteRepositoryRequest{
				RepoRelativePath: repo.RelativePath(),
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return err
	}
	return util.RemoveAll(repoPath(repo))
}

// RenameRepository renames a repository's name on disk
func RenameRepository(ctx context.Context, repo, newRepo Repository) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.RenameRepository(cctx, &gitstoragev1.RenameRepositoryRequest{
				FromRepoRelativePath: repo.RelativePath(),
				ToRepoRelativePath:   newRepo.RelativePath(),
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
			invalidateRemoteMirror(newRepo.RelativePath())
		}
		return err
	}
	dstDir := repoPath(newRepo)
	if err := os.MkdirAll(filepath.Dir(dstDir), os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %w", filepath.Dir(dstDir), err)
	}

	if err := util.Rename(repoPath(repo), dstDir); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	return nil
}

func InitRepository(ctx context.Context, repo Repository, objectFormatName string) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.InitRepository(cctx, &gitstoragev1.InitRepositoryRequest{
				RepoRelativePath: repo.RelativePath(),
				ObjectFormatName: objectFormatName,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return err
	}
	return git.InitRepository(ctx, repoPath(repo), true, objectFormatName)
}

func UpdateServerInfo(ctx context.Context, repo Repository) error {
	_, _, err := RunCmdBytes(ctx, repo, gitcmd.NewCommand("update-server-info"))
	return err
}

func GetRepoFS(repo Repository) fs.FS {
	if isRemoteBackendEnabled() {
		ctx := context.Background()
		localPath, err := ensureRemoteMirror(ctx, repo)
		if err == nil {
			return os.DirFS(localPath)
		}
	}
	return os.DirFS(repoPath(repo))
}

func IsRepoFileExist(ctx context.Context, repo Repository, relativeFilePath string) (bool, error) {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return false, err
		}
		exists := false
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			resp, err := client.StatRepoPath(cctx, &gitstoragev1.StatRepoPathRequest{
				RepoRelativePath: repo.RelativePath(),
				RelativePath:     relativeFilePath,
			})
			if err != nil {
				return err
			}
			exists = resp.GetExists() && !resp.GetIsDir()
			return nil
		})
		return exists, err
	}
	absoluteFilePath := filepath.Join(repoPath(repo), relativeFilePath)
	return util.IsExist(absoluteFilePath)
}

func IsRepoDirExist(ctx context.Context, repo Repository, relativeDirPath string) (bool, error) {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return false, err
		}
		exists := false
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			resp, err := client.StatRepoPath(cctx, &gitstoragev1.StatRepoPathRequest{
				RepoRelativePath: repo.RelativePath(),
				RelativePath:     relativeDirPath,
			})
			if err != nil {
				return err
			}
			exists = resp.GetExists() && resp.GetIsDir()
			return nil
		})
		return exists, err
	}
	absoluteDirPath := filepath.Join(repoPath(repo), relativeDirPath)
	return util.IsDir(absoluteDirPath)
}

func RemoveRepoFileOrDir(ctx context.Context, repo Repository, relativeFileOrDirPath string) error {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return err
		}
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			_, err := client.RemoveRepoPath(cctx, &gitstoragev1.RemoveRepoPathRequest{
				RepoRelativePath: repo.RelativePath(),
				RelativePath:     relativeFileOrDirPath,
			})
			return err
		})
		if err == nil {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return err
	}
	absoluteFilePath := filepath.Join(repoPath(repo), relativeFileOrDirPath)
	return util.Remove(absoluteFilePath)
}

func CreateRepoFile(ctx context.Context, repo Repository, relativeFilePath string) (io.WriteCloser, error) {
	return CreateRepoFileWithMode(ctx, repo, relativeFilePath, 0o644)
}

func CreateRepoFileWithMode(ctx context.Context, repo Repository, relativeFilePath string, fileMode os.FileMode) (io.WriteCloser, error) {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return nil, err
		}
		return &remoteRepoFileWriter{
			ctx:      ctx,
			repo:     repo.RelativePath(),
			relative: relativeFilePath,
			client:   client,
			fileMode: uint32(fileMode.Perm()),
		}, nil
	}
	absoluteFilePath := filepath.Join(repoPath(repo), relativeFilePath)
	if err := os.MkdirAll(filepath.Dir(absoluteFilePath), os.ModePerm); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(absoluteFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode.Perm())
	if err != nil {
		return nil, err
	}
	return f, nil
}

type remoteRepoFileWriter struct {
	ctx      context.Context
	repo     string
	relative string
	client   gitstoragev1.GitStorageClient
	fileMode uint32
	buf      bytes.Buffer
	closed   bool
}

func (w *remoteRepoFileWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fs.ErrClosed
	}
	return w.buf.Write(p)
}

func (w *remoteRepoFileWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	err := callRemoteWithTimeout(w.ctx, func(cctx context.Context) error {
		_, err := w.client.WriteRepoFile(cctx, &gitstoragev1.WriteRepoFileRequest{
			RepoRelativePath: w.repo,
			RelativePath:     w.relative,
			Content:          w.buf.Bytes(),
			FileMode:         w.fileMode,
		})
		return err
	})
	if err == nil {
		invalidateRemoteMirror(w.repo)
	}
	return err
}

func ensureRemoteMirror(ctx context.Context, repo Repository) (string, error) {
	client, err := getRemoteClient()
	if err != nil {
		return "", err
	}
	localPath := remoteMirrorPath(repo.RelativePath())
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return "", err
	}
	if shouldReuseRemoteMirror(localPath) {
		return localPath, nil
	}
	if err := hydrateRemoteMirrorViaBundle(ctx, client, repo.RelativePath(), localPath); err != nil {
		return "", err
	}
	_ = touchRemoteMirror(localPath)
	return localPath, nil
}

func hydrateRemoteMirrorViaBundle(ctx context.Context, client gitstoragev1.GitStorageClient, repoRelative, localPath string) error {
	var resp *gitstoragev1.RunGitCommandResponse
	err := callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.RunGitCommand(cctx, &gitstoragev1.RunGitCommandRequest{
			RepoRelativePath: repoRelative,
			Args:             []string{"bundle", "create", "-", "--all"},
		})
		return reqErr
	})
	if err != nil {
		return err
	}

	// Empty repos cannot produce a bundle; initialize an empty mirror locally.
	if resp.GetExecError() != "" {
		if strings.Contains(resp.GetExecError(), "Refusing to create empty bundle") ||
			strings.Contains(string(resp.GetStderr()), "Refusing to create empty bundle") {
			_ = os.RemoveAll(localPath)
			return gitcmd.NewCommand("init", "--bare").AddDynamicArguments(localPath).Run(ctx)
		}
		return fmt.Errorf("bundle export failed: %s (%s)", resp.GetExecError(), string(resp.GetStderr()))
	}
	if len(resp.GetStdout()) == 0 {
		return fmt.Errorf("bundle export failed: empty payload")
	}

	bundleFile, err := os.CreateTemp(filepath.Dir(localPath), "git-storage-*.bundle")
	if err != nil {
		return err
	}
	bundlePath := bundleFile.Name()
	defer func() {
		_ = bundleFile.Close()
		_ = os.Remove(bundlePath)
	}()
	if _, err := bundleFile.Write(resp.GetStdout()); err != nil {
		return err
	}
	if err := bundleFile.Close(); err != nil {
		return err
	}

	_ = os.RemoveAll(localPath)
	if err := gitcmd.NewCommand("clone", "--mirror").AddDashesAndList(bundlePath, localPath).Run(ctx); err != nil {
		return err
	}
	return nil
}

func remoteMirrorPath(relative string) string {
	root := strings.TrimSpace(os.Getenv("GIT_STORAGE_LOCAL_CACHE_ROOT"))
	if root == "" {
		root = filepath.Join(os.TempDir(), "core-scm-git-storage-cache")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	return filepath.Join(root, clean)
}

func invalidateRemoteMirror(relative string) {
	_ = os.RemoveAll(remoteMirrorPath(relative))
}

func shouldReuseRemoteMirror(localPath string) bool {
	ttl := remoteMirrorTTL()
	if ttl <= 0 {
		return false
	}
	info, err := os.Stat(localPath)
	if err != nil || !info.IsDir() {
		return false
	}
	return time.Since(info.ModTime()) <= ttl
}

func remoteMirrorTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GIT_STORAGE_LOCAL_CACHE_TTL"))
	if raw == "" {
		return 3 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 3 * time.Second
	}
	return d
}

func touchRemoteMirror(localPath string) error {
	now := time.Now()
	return os.Chtimes(localPath, now, now)
}
