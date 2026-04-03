package gitrepo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strconv"
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
		conn, err := grpc.NewClient(
			endpoint,
			grpc.WithTransportCredentials(transportCreds),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(gitStorageMaxRecvMessageBytes()),
			),
		)
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

func gitStorageMaxRecvMessageBytes() int {
	// gRPC default is 4MiB, which is too small for larger receive-pack responses.
	const def = 64 << 20 // 64MiB
	raw := strings.TrimSpace(os.Getenv("GIT_STORAGE_GRPC_MAX_RECV_BYTES"))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
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
