// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/gitprotocontext"
	"github.com/google/gitprotocolio"
)

func fetchPackfile(ctx context.Context, repoURL string, client *http.Client, body []byte) ([]byte, debug.FetchDebugInfo, error) {
	rd, headers, err := callProtocolV2(ctx, repoURL, client, body)
	debugInfo := debug.FetchDebugInfo{ResponseHeaders: headers}
	if err != nil {
		return nil, debugInfo, err
	}
	defer rd.Close()
	v2Resp := gitprotocolio.NewProtocolV2Response(rd)
	isPackfile := false
	packfile := bytes.NewBuffer(nil)
	for v2Resp.Scan() {
		chunk := v2Resp.Chunk()
		if chunk.EndResponse {
			break
		}
		if chunk.Delimiter {
			continue
		}
		if isPackfile {
			sideband := gitprotocolio.ParseSideBandPacket(chunk.Response)
			if sideband == nil {
				return nil, debugInfo, errors.New("unexpected non-sideband packet")
			}
			if pkt, ok := sideband.(gitprotocolio.SideBandMainPacket); ok {
				packfile.Write(pkt.Bytes())
			}
			continue
		}
		if bytes.Equal(chunk.Response, []byte("shallow-info\n")) {
			// No use. Skipping.
			continue
		}
		if bytes.HasPrefix(chunk.Response, []byte("shallow ")) {
			// No use. Skipping.
			continue
		}
		if bytes.Equal(chunk.Response, []byte("packfile\n")) {
			isPackfile = true
			continue
		}
	}
	if err := v2Resp.Err(); err != nil {
		return nil, debugInfo, fmt.Errorf("failed to parse the protov2 response: %v", err)
	}
	debugInfo.PackfileSize = packfile.Len()
	return packfile.Bytes(), debugInfo, nil
}

func callProtocolV2(ctx context.Context, repoURL string, client *http.Client, body []byte) (io.ReadCloser, http.Header, error) {
	retryCount := gitprotocontext.GitFetchRetryCount(ctx)
	var errs []error
	for {
		childCtx, cancel := ctx, func() {}
		if timeout := gitprotocontext.GitFetchTimeout(ctx); timeout > 0 {
			childCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		switch {
		case strings.HasPrefix(repoURL, "http"):
			rd, headers, err := callProtocolV2HTTP(childCtx, repoURL, client, body)
			cancel()
			if err == nil {
				return rd, headers, nil
			}
			errs = append(errs, err)
		case strings.HasPrefix(repoURL, "file"):
			rd, err := callProtocolV2File(childCtx, repoURL, body)
			cancel()
			if err == nil {
				return rd, http.Header{}, nil
			}
			errs = append(errs, err)
		default:
			cancel()
			return nil, nil, errors.New("unsupported protocol")
		}

		retryCount--
		if retryCount <= 0 {
			return nil, nil, errors.Join(errs...)
		}
	}
}

func callProtocolV2HTTP(ctx context.Context, repoURL string, client *http.Client, body []byte) (io.ReadCloser, http.Header, error) {
	upURL, err := buildUploadPackURL(repoURL)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	req.Header.Set("Git-Protocol", "version=2")
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Header, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return resp.Body, resp.Header, nil
}

func callProtocolV2File(ctx context.Context, repoURL string, body []byte) (io.ReadCloser, error) {
	fpath := strings.TrimPrefix(repoURL, "file://")
	cmd := exec.CommandContext(ctx, "git", "-c", "uploadpack.allowFilter=1", "upload-pack", "--stateless-rpc", fpath)
	cmd.Stdin = bytes.NewReader(body)
	cmd.Stderr = os.Stderr
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Env = append(cmd.Env, "GIT_PROTOCOL=version=2")
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return io.NopCloser(stdout), nil
}

func buildUploadPackURL(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	u = u.JoinPath("git-upload-pack")
	return u.String(), nil
}
