package gitrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateDelegateHooksRemoteSmoke(t *testing.T) {
	endpoint := os.Getenv("GIT_STORAGE_SMOKE_ENDPOINT")
	if endpoint == "" {
		t.Skip("set GIT_STORAGE_SMOKE_ENDPOINT to run remote hooks smoke")
	}

	t.Setenv("GIT_STORAGE_BACKEND", "remote")
	t.Setenv("GIT_STORAGE_ENDPOINT", endpoint)
	t.Setenv("GIT_STORAGE_TIMEOUTS", "30s")
	t.Setenv("GIT_STORAGE_LOCAL_CACHE_ROOT", t.TempDir())

	repo := &mockRepository{path: fmt.Sprintf("smoke-hooks/%d.git", time.Now().UnixNano())}
	ctx := context.Background()

	require.NoError(t, InitRepository(ctx, repo, "sha1"))
	t.Cleanup(func() {
		_ = DeleteRepository(context.Background(), repo)
	})

	require.NoError(t, CreateDelegateHooks(ctx, repo))

	issues, err := CheckDelegateHooks(ctx, repo)
	require.NoError(t, err)
	require.Empty(t, issues, "hook templates should be up-to-date")

	exists, err := IsRepoFileExist(ctx, repo, "hooks/pre-receive")
	require.NoError(t, err)
	require.True(t, exists)

	localPath, err := ensureRemoteMirror(ctx, repo)
	require.NoError(t, err)

	st, err := os.Stat(filepath.Join(localPath, "hooks", "pre-receive"))
	require.NoError(t, err)
	require.NotZero(t, st.Mode()&0o100, "hook file must be executable")
}
