// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package fetch

import (
	"bytes"
	"net/http"

	"github.com/google/gitprotocolio"
)

// FetchBlobNonePackfile fetches a packfile from a remote repository without blobs.
func FetchBlobNonePackfile(repoURL string, client *http.Client, oids []string) ([]byte, http.Header, error) {
	return fetchPackfile(repoURL, client, createBlobNoneFetchRequest(oids))
}

func createBlobNoneFetchRequest(oids []string) *bytes.Buffer {
	chunks := []*gitprotocolio.ProtocolV2RequestChunk{
		{
			Command: "fetch",
		},
		{
			EndCapability: true,
		},
	}
	for _, oid := range oids {
		chunks = append(chunks, &gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("want " + oid),
		})
	}
	chunks = append(chunks,
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("no-progress"),
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("deepen 1"),
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("filter blob:none"),
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
