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
	"github.com/google/gitprotocolio"
)

func fetchPackfile(ctx context.Context, repoURL string, client *http.Client, body *bytes.Buffer) ([]byte, debug.FetchDebugInfo, error) {
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

func callProtocolV2(ctx context.Context, repoURL string, client *http.Client, body *bytes.Buffer) (io.ReadCloser, http.Header, error) {
	if strings.HasPrefix(repoURL, "http") {
		return callProtocolV2HTTP(ctx, repoURL, client, body)
	} else if strings.HasPrefix(repoURL, "file") {
		rd, err := callProtocolV2File(ctx, repoURL, body)
		return rd, http.Header{}, err
	}
	return nil, nil, errors.New("unsupported protocol")
}

func callProtocolV2HTTP(ctx context.Context, repoURL string, client *http.Client, body *bytes.Buffer) (io.ReadCloser, http.Header, error) {
	upURL, err := buildUploadPackURL(repoURL)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upURL, body)
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

func callProtocolV2File(ctx context.Context, repoURL string, body *bytes.Buffer) (io.ReadCloser, error) {
	fpath := strings.TrimPrefix(repoURL, "file://")
	cmd := exec.CommandContext(ctx, "git", "-c", "uploadpack.allowFilter=1", "upload-pack", "--stateless-rpc", fpath)
	cmd.Stdin = body
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
