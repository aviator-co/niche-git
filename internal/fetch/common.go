// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package fetch

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/gitprotocolio"
)

func fetchPackfile(repoURL string, client *http.Client, body *bytes.Buffer) ([]byte, http.Header, error) {
	upURL, err := buildUploadPackURL(repoURL)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest("POST", upURL, body)
	if err != nil {
		return nil, nil, err
	}
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
	v2Resp := gitprotocolio.NewProtocolV2Response(resp.Body)
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
				return nil, resp.Header, errors.New("unexpected non-sideband packet")
			}
			if pkt, ok := sideband.(gitprotocolio.BytePayloadPacket); ok {
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
		return nil, resp.Header, fmt.Errorf("failed to parse the protov2 resposne: %v", err)

	}
	return packfile.Bytes(), resp.Header, v2Resp.Err()
}

func buildUploadPackURL(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	u = u.JoinPath("git-upload-pack")
	return u.String(), nil
}
