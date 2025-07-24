// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package fetch

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/aviator-co/niche-git/debug"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/gitprotocolio"
)

// FetchBlobNonePackfile fetches a packfile from a remote repository without blobs.
func FetchBlobNonePackfile(ctx context.Context, repoURL string, client *http.Client, oids []plumbing.Hash, depth int) ([]byte, debug.FetchDebugInfo, error) {
	return fetchPackfile(ctx, repoURL, client, createBlobNoneFetchRequest(oids, depth))
}

func createBlobNoneFetchRequest(oids []plumbing.Hash, depth int) []byte {
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
			Argument: []byte("want " + oid.String()),
		})
	}
	chunks = append(chunks,
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte("no-progress"),
		},
		&gitprotocolio.ProtocolV2RequestChunk{
			Argument: []byte(fmt.Sprintf("deepen %d", depth)),
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
	return bs.Bytes()
}
