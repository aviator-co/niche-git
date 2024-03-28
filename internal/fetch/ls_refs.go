// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package fetch

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/google/gitprotocolio"
)

// LsRefs fetches a refs from a remote repository.
func LsRefs(repoURL string, client *http.Client, refPrefixes []string) ([]string, http.Header, error) {
	upURL, err := buildUploadPackURL(repoURL)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest("POST", upURL, createLsRefsRequest(refPrefixes))
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
	var refData []string
	for v2Resp.Scan() {
		chunk := v2Resp.Chunk()
		if chunk.EndResponse {
			break
		}
		refData = append(refData, string(chunk.Response))
	}
	if err := v2Resp.Err(); err != nil {
		return nil, resp.Header, fmt.Errorf("failed to parse the protov2 resposne: %v", err)

	}
	return refData, resp.Header, v2Resp.Err()
}

func createLsRefsRequest(refPrefixes []string) *bytes.Buffer {
	chunks := []*gitprotocolio.ProtocolV2RequestChunk{
		{
			Command: "ls-refs",
		},
		{
			EndCapability: true,
		},
	}
	for _, refPrefix := range refPrefixes {
		chunks = append(chunks, &gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("ref-prefix " + refPrefix),
		})
	}
	chunks = append(chunks,
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("symrefs"),
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("peel"),
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			EndArgument: true,
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			EndRequest: true,
		},
	)
	bs := bytes.NewBuffer(nil)
	for _, chunk := range chunks {
		// Not possible to fail.
		bs.Write(chunk.EncodeToPktLine())
	}
	return bs
}
