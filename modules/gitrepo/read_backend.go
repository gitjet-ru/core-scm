package gitrepo

import (
	"context"
	"os"
	"strings"

	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

func readBackendMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GIT_STORAGE_READ_BACKEND")))
	if mode == "" {
		if isRemoteBackendEnabled() {
			return "remote"
		}
		return "local"
	}
	return mode
}

func useRemoteReadBackend() bool {
	if !isRemoteBackendEnabled() {
		return false
	}
	mode := readBackendMode()
	return mode == "remote" || mode == "shadow"
}

// UseRemoteReadBackendForAPI reports whether API read-path should bypass local mirror.
func UseRemoteReadBackendForAPI() bool {
	return useRemoteReadBackend()
}

func remoteListRefs(ctx context.Context, repo Repository, prefix string) ([]*gitstoragev1.RefInfo, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, err
	}
	var resp *gitstoragev1.ListRefsResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.ListRefs(cctx, &gitstoragev1.ListRefsRequest{
			RepoRelativePath: repo.RelativePath(),
			Prefix:           prefix,
		})
		return reqErr
	})
	if err != nil {
		return nil, err
	}
	return resp.GetRefs(), nil
}

func RemoteListRefsForAPI(ctx context.Context, repo Repository, prefix string) ([]*gitstoragev1.RefInfo, error) {
	return remoteListRefs(ctx, repo, prefix)
}

func remoteGetTree(ctx context.Context, repo Repository, ref, path string, recursive bool, maxEntries int32) ([]*gitstoragev1.TreeEntry, bool, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, false, err
	}
	var resp *gitstoragev1.GetTreeResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.GetTree(cctx, &gitstoragev1.GetTreeRequest{
			RepoRelativePath: repo.RelativePath(),
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

func RemoteGetTreeForAPI(ctx context.Context, repo Repository, ref, path string, recursive bool, maxEntries int32) ([]*gitstoragev1.TreeEntry, bool, error) {
	return remoteGetTree(ctx, repo, ref, path, recursive, maxEntries)
}

func remoteGetBlob(ctx context.Context, repo Repository, ref, path string, maxBytes int32) (*gitstoragev1.GetBlobResponse, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, err
	}
	var resp *gitstoragev1.GetBlobResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.GetBlob(cctx, &gitstoragev1.GetBlobRequest{
			RepoRelativePath: repo.RelativePath(),
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

func RemoteGetBlobForAPI(ctx context.Context, repo Repository, ref, path string, maxBytes int32) (*gitstoragev1.GetBlobResponse, error) {
	return remoteGetBlob(ctx, repo, ref, path, maxBytes)
}

func remoteGetCommit(ctx context.Context, repo Repository, rev string) (*gitstoragev1.CommitInfo, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, err
	}
	var resp *gitstoragev1.GetCommitResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.GetCommit(cctx, &gitstoragev1.GetCommitRequest{
			RepoRelativePath: repo.RelativePath(),
			Rev:              rev,
		})
		return reqErr
	})
	if err != nil {
		return nil, err
	}
	return resp.GetCommit(), nil
}

func RemoteGetCommitForAPI(ctx context.Context, repo Repository, rev string) (*gitstoragev1.CommitInfo, error) {
	return remoteGetCommit(ctx, repo, rev)
}

func remoteListCommits(ctx context.Context, repo Repository, ref string, limit, skip int32, relPath, since, until, not string) ([]*gitstoragev1.CommitInfo, error) {
	client, err := getRemoteClient()
	if err != nil {
		return nil, err
	}
	var resp *gitstoragev1.ListCommitsResponse
	err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
		var reqErr error
		resp, reqErr = client.ListCommits(cctx, &gitstoragev1.ListCommitsRequest{
			RepoRelativePath: repo.RelativePath(),
			Ref:              ref,
			Limit:            limit,
			Skip:             skip,
			Path:             relPath,
			Since:            since,
			Until:            until,
			Not:              not,
		})
		return reqErr
	})
	if err != nil {
		return nil, err
	}
	return resp.GetCommits(), nil
}

func RemoteListCommitsForAPI(ctx context.Context, repo Repository, ref string, limit, skip int32, relPath, since, until, not string) ([]*gitstoragev1.CommitInfo, error) {
	return remoteListCommits(ctx, repo, ref, limit, skip, relPath, since, until, not)
}
