package gitrepo

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	remoteClientOnce sync.Once
	remoteClient     gitstoragev1.GitStorageClient
	remoteClientErr  error
)

func isRemoteBackendEnabled() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GIT_STORAGE_BACKEND")))
	return mode == "remote" || mode == "shadow"
}

func getRemoteClient() (gitstoragev1.GitStorageClient, error) {
	remoteClientOnce.Do(func() {
		endpoint := strings.TrimSpace(os.Getenv("GIT_STORAGE_ENDPOINT"))
		if endpoint == "" {
			remoteClientErr = errors.New("GIT_STORAGE_ENDPOINT is not set")
			return
		}
		conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			remoteClientErr = err
			return
		}
		remoteClient = gitstoragev1.NewGitStorageClient(conn)
	})
	return remoteClient, remoteClientErr
}

func callRemoteWithTimeout(ctx context.Context, fn func(context.Context) error) error {
	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("GIT_STORAGE_TIMEOUTS")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			timeout = d
		}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(cctx)
}
