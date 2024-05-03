// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package fetch

import (
	"bytes"
	"net/http"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/gitprotocolio"
)

// FetchCommitOnlyPackfile fetches a packfile from a remote repository with only commit objects.
func FetchCommitOnlyPackfile(repoURL string, client *http.Client, wantOids, haveOids []plumbing.Hash) ([]byte, http.Header, error) {
	return fetchPackfile(repoURL, client, createCommitOnlyFetchRequest(wantOids, haveOids))
}

func createCommitOnlyFetchRequest(wantOids, haveOids []plumbing.Hash) *bytes.Buffer {
	chunks := []*gitprotocolio.ProtocolV2RequestChunk{
		{
			Command: "fetch",
		},
		{
			EndCapability: true,
		},
	}
	for _, oid := range wantOids {
		chunks = append(chunks, &gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("want " + oid.String()),
		})
	}
	for _, oid := range haveOids {
		chunks = append(chunks, &gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("have " + oid.String()),
		})
	}
	chunks = append(chunks,
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("no-progress"),
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("filter tree:0"),
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("done"),
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
