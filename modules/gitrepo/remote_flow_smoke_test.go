package gitrepo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitjet-ru/core-scm/modules/git"
	"github.com/stretchr/testify/require"
)

func TestRemotePRWikiReleaseFlowsSmoke(t *testing.T) {
	endpoint := os.Getenv("GIT_STORAGE_SMOKE_ENDPOINT")
	if endpoint == "" {
		t.Skip("set GIT_STORAGE_SMOKE_ENDPOINT to run remote flow smoke")
	}

	t.Setenv("GIT_STORAGE_BACKEND", "remote")
	t.Setenv("GIT_STORAGE_ENDPOINT", endpoint)
	t.Setenv("GIT_STORAGE_TIMEOUTS", "30s")
	t.Setenv("GIT_STORAGE_LOCAL_CACHE_ROOT", t.TempDir())
	t.Setenv("GIT_STORAGE_LOCAL_CACHE_TTL", "1s")

	ctx := context.Background()
	repo := &mockRepository{path: fmt.Sprintf("smoke-flows/%d.git", time.Now().UnixNano())}
	wikiRepo := &mockRepository{path: fmt.Sprintf("smoke-flows/%d.wiki.git", time.Now().UnixNano())}

	require.NoError(t, InitRepository(ctx, repo, "sha1"))
	require.NoError(t, InitRepository(ctx, wikiRepo, "sha1"))
	t.Cleanup(func() {
		_ = DeleteRepository(context.Background(), repo)
		_ = DeleteRepository(context.Background(), wikiRepo)
	})

	local := t.TempDir()
	runGitCmd(t, local, "init")
	runGitCmd(t, local, "config", "user.name", "Smoke")
	runGitCmd(t, local, "config", "user.email", "smoke@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(local, "README.md"), []byte("hello remote"), 0o644))
	runGitCmd(t, local, "add", "README.md")
	runGitCmd(t, local, "commit", "-m", "init")

	require.NoError(t, PushFromLocal(ctx, local, repo, git.PushOptions{
		LocalRefName: "refs/heads/master",
		Branch:       "main",
		Force:        true,
	}))
	require.NoError(t, SetDefaultBranch(ctx, repo, "main"))

	mainCommit, err := GetBranchCommitID(ctx, repo, "main")
	require.NoError(t, err)
	require.NotEmpty(t, mainCommit)

	require.NoError(t, CreateBranch(ctx, repo, "feature/smoke", "main"))
	featureCommit, err := GetBranchCommitID(ctx, repo, "feature/smoke")
	require.NoError(t, err)

	mergeBase, err := MergeBase(ctx, repo, mainCommit, featureCommit)
	require.NoError(t, err)
	require.Equal(t, mainCommit, mergeBase)

	var archive bytes.Buffer
	require.NoError(t, CreateArchive(ctx, repo, "tar", &archive, true, mainCommit, nil))
	require.NotZero(t, archive.Len())

	var bundle bytes.Buffer
	require.NoError(t, CreateBundle(ctx, repo, mainCommit, &bundle))
	require.NotZero(t, bundle.Len())

	require.NoError(t, CreateDelegateHooks(ctx, wikiRepo))
	wikiIssues, err := CheckDelegateHooks(ctx, wikiRepo)
	require.NoError(t, err)
	require.Empty(t, wikiIssues)
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, string(out))
}
