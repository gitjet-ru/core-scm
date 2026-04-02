package git

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

func remoteReadBackendMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GIT_STORAGE_READ_BACKEND")))
	if mode == "" {
		if isRemoteBackendEnabled() {
			return "remote"
		}
		return "local"
	}
	return mode
}

func remoteReadsEnabled() bool {
	if !isRemoteBackendEnabled() {
		return false
	}
	mode := remoteReadBackendMode()
	return mode == "remote" || mode == "shadow"
}

// remoteReadRepo returns true when the repository should be accessed via git-storage RPC.
// We treat non-absolute repo.Path values as "remote-only" identifiers (e.g. "owner/repo.git").
func remoteReadRepo(repo *Repository) bool {
	return remoteReadsEnabled() && repo != nil && !filepath.IsAbs(repo.Path)
}

func getRemoteClient() (gitstoragev1.GitStorageClient, error) {
	remoteClientOnce.Do(func() {
		endpoint := strings.TrimSpace(os.Getenv("GIT_STORAGE_ENDPOINT"))
		if endpoint == "" {
			remoteClientErr = errors.New("GIT_STORAGE_ENDPOINT is not set")
			return
		}
		transportCreds, err := gitStorageTransportCredentials()
		if err != nil {
			remoteClientErr = err
			return
		}
		conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(transportCreds))
		if err != nil {
			remoteClientErr = err
			return
		}
		remoteClient = gitstoragev1.NewGitStorageClient(conn)
	})
	return remoteClient, remoteClientErr
}

func gitStorageTransportCredentials() (credentials.TransportCredentials, error) {
	caFile := strings.TrimSpace(os.Getenv("GIT_STORAGE_TLS_CA_FILE"))
	certFile := strings.TrimSpace(os.Getenv("GIT_STORAGE_TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("GIT_STORAGE_TLS_KEY_FILE"))
	serverName := strings.TrimSpace(os.Getenv("GIT_STORAGE_TLS_SERVER_NAME"))
	if caFile == "" && certFile == "" && keyFile == "" {
		return insecure.NewCredentials(), nil
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if serverName != "" {
		tlsConfig.ServerName = serverName
	}
	if caFile != "" {
		caBytes, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, errors.New("failed to parse GIT_STORAGE_TLS_CA_FILE")
		}
		tlsConfig.RootCAs = pool
	}
	if certFile != "" || keyFile != "" {
		if certFile == "" || keyFile == "" {
			return nil, errors.New("both GIT_STORAGE_TLS_CERT_FILE and GIT_STORAGE_TLS_KEY_FILE are required")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return credentials.NewTLS(tlsConfig), nil
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

func remoteGetTree(ctx context.Context, repoRelativePath, ref, path string, recursive bool, maxEntries int32) ([]*gitstoragev1.TreeEntry, bool, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, false, err
	}

	var resp *gitstoragev1.GetTreeResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.GetTree(cctx, &gitstoragev1.GetTreeRequest{
			RepoRelativePath: repoRelativePath,
			Ref:              ref,
			Path:             path,
			Recursive:        recursive,
			MaxEntries:       maxEntries,
		})
		return reqErr
	})
	if err != nil {
		return nil, false, err
	}
	return resp.GetEntries(), resp.GetTruncated(), nil
}

func remoteGetBlob(ctx context.Context, repoRelativePath, ref, path string, maxBytes int32) (*gitstoragev1.GetBlobResponse, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, err
	}

	var resp *gitstoragev1.GetBlobResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.GetBlob(cctx, &gitstoragev1.GetBlobRequest{
			RepoRelativePath: repoRelativePath,
			Ref:              ref,
			Path:             path,
			MaxBytes:         maxBytes,
		})
		return reqErr
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func remoteGetCommit(ctx context.Context, repoRelativePath, rev string) (*gitstoragev1.CommitInfo, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, err
	}

	var resp *gitstoragev1.GetCommitResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.GetCommit(cctx, &gitstoragev1.GetCommitRequest{
			RepoRelativePath: repoRelativePath,
			Rev:              rev,
		})
		return reqErr
	})
	if err != nil {
		return nil, err
	}
	return resp.GetCommit(), nil
}

