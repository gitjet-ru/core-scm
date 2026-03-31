package gitrepo

import (
	"context"
	"strings"

	gitstoragev1 "github.com/gitjet-ru/git-storage/gen/go/gitstorage/v1"
)

// RunSmartService executes stateless git smart service call.
// service should be one of: upload-pack, receive-pack, upload-archive.
func RunSmartService(ctx context.Context, repo Repository, service string, stdin []byte, env []string) ([]byte, []byte, int32, error) {
	if isRemoteBackendEnabled() {
		client, err := getRemoteClient()
		if err != nil {
			return nil, nil, -1, err
		}
		var out *gitstoragev1.SmartServiceResponse
		err = callRemoteWithTimeout(ctx, func(cctx context.Context) error {
			resp, err := client.SmartService(cctx, &gitstoragev1.SmartServiceRequest{
				RepoRelativePath: repo.RelativePath(),
				Service:          service,
				Stdin:            stdin,
				Env:              env,
			})
			if err != nil {
				return err
			}
			out = resp
			return nil
		})
		if err != nil {
			return nil, nil, -1, err
		}
		if strings.EqualFold(service, "receive-pack") {
			invalidateRemoteMirror(repo.RelativePath())
		}
		return out.GetStdout(), out.GetStderr(), out.GetExitCode(), nil
	}

	return nil, nil, -1, nil
}
