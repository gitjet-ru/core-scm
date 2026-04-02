package gitrepo

import (
	"context"
	"io"
	"strings"
	"sync"

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

// RunSmartServiceStream proxies a full-duplex smart-service session.
func RunSmartServiceStream(ctx context.Context, repo Repository, service string, env []string, in io.Reader, out, errOut io.Writer) (int32, error) {
	if !isRemoteBackendEnabled() {
		return -1, nil
	}
	client, err := getRemoteClient()
	if err != nil {
		return -1, err
	}
	stream, err := client.SmartServiceStream(ctx)
	if err != nil {
		return -1, err
	}
	if err := stream.Send(&gitstoragev1.SmartServiceStreamRequest{
		RepoRelativePath: repo.RelativePath(),
		Service:          service,
		Env:              env,
	}); err != nil {
		return -1, err
	}

	sendErrCh := make(chan error, 1)
	go func() {
		const chunkSize = 32 * 1024
		buf := make([]byte, chunkSize)
		for {
			n, readErr := in.Read(buf)
			if n > 0 {
				if err := stream.Send(&gitstoragev1.SmartServiceStreamRequest{
					StdinChunk: append([]byte(nil), buf[:n]...),
				}); err != nil {
					sendErrCh <- err
					return
				}
			}
			if readErr == io.EOF {
				sendErrCh <- stream.Send(&gitstoragev1.SmartServiceStreamRequest{Eof: true})
				return
			}
			if readErr != nil {
				sendErrCh <- readErr
				return
			}
		}
	}()

	var recvMu sync.Mutex
	exitCode := int32(0)
	for {
		resp, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			return -1, recvErr
		}
		recvMu.Lock()
		if len(resp.GetStdoutChunk()) > 0 {
			if _, err := out.Write(resp.GetStdoutChunk()); err != nil {
				recvMu.Unlock()
				return -1, err
			}
		}
		if len(resp.GetStderrChunk()) > 0 {
			if _, err := errOut.Write(resp.GetStderrChunk()); err != nil {
				recvMu.Unlock()
				return -1, err
			}
		}
		if resp.GetDone() {
			exitCode = resp.GetExitCode()
			recvMu.Unlock()
			break
		}
		recvMu.Unlock()
	}

	if sendErr := <-sendErrCh; sendErr != nil {
		return -1, sendErr
	}
	if strings.EqualFold(service, "receive-pack") && exitCode == 0 {
		invalidateRemoteMirror(repo.RelativePath())
	}
	return exitCode, nil
}
