// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package fetch

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/google/gitprotocolio"
)

// LsRefs fetches a refs from a remote repository.
func LsRefs(ctx context.Context, repoURL string, client *http.Client, refPrefixes []string) ([]string, http.Header, error) {
	var refData []string
	retHeaders := &http.Header{}
	err := callProtocolV2(ctx, repoURL, client, createLsRefsRequest(refPrefixes), func(headers http.Header, rd io.Reader) error {
		*retHeaders = headers
		v2Resp := gitprotocolio.NewProtocolV2Response(rd)
		isServerInfo := false
		for v2Resp.Scan() {
			chunk := v2Resp.Chunk()
			if chunk.EndResponse {
				if isServerInfo {
					isServerInfo = false
					continue
				}
				break
			}
			if bytes.Equal(chunk.Response, []byte("version 2\n")) {
				isServerInfo = true
				continue
			}
			if isServerInfo {
				continue
			}
			refData = append(refData, string(chunk.Response))
		}
		return nil
	})
	if err != nil {
		return nil, *retHeaders, err
	}
	return refData, *retHeaders, nil
}

func createLsRefsRequest(refPrefixes []string) []byte {
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
	return bs.Bytes()
}
