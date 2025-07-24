// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"bytes"
	"context"
	"net/http"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/push"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/storage/memory"
)

type RefUpdateCommand struct {
	// RefName is a reference name to update (e.g. "refs/heads/main").
	RefName string `json:"refName"`
	// OldHash is a hash of the reference before the update.
	//
	// There is a difference between zero hash and empty string:
	//
	// * If this is a zero hash, it means that the reference should be newly created (it should
	//   not exist.
	// * If this is an empty string, it means that the reference is updated unconditionally
	//   (force update).
	//
	// Note that, at the git-transport level, everything is a force update. The client should
	// check if the reference being updated is fast-forwardable if they want such behavior.
	OldHash string `json:"oldHash"`
	// NewHash is a hash of the reference after the update.
	NewHash string `json:"newHash"`
}

type UpdateRefsArgs struct {
	RepoURL           string             `json:"repoURL"`
	RefUpdateCommands []RefUpdateCommand `json:"refUpdateCommands"`
}

type UpdateRefsOutput struct {
	PushDebugInfo *debug.PushDebugInfo `json:"pushDebugInfo"`
	Error         string               `json:"error,omitempty"`
}

func UpdateRefs(ctx context.Context, client *http.Client, args UpdateRefsArgs) UpdateRefsOutput {
	var refUpdates []push.RefUpdate
	for _, refUpdateCommand := range args.RefUpdateCommands {
		ru := push.RefUpdate{
			Name:    plumbing.ReferenceName(refUpdateCommand.RefName),
			OldHash: nil,
			NewHash: plumbing.NewHash(refUpdateCommand.NewHash),
		}
		if refUpdateCommand.OldHash != "" {
			h := plumbing.NewHash(refUpdateCommand.OldHash)
			ru.OldHash = &h
		}
		refUpdates = append(refUpdates, ru)
	}

	// Need to create an empty packfile to push.
	var buf bytes.Buffer
	packEncoder := packfile.NewEncoder(&buf, memory.NewStorage(), false)
	if _, err := packEncoder.Encode(nil, 0); err != nil {
		return UpdateRefsOutput{Error: err.Error()}
	}

	var ret UpdateRefsOutput
	pushDebugInfo, err := push.Push(ctx, args.RepoURL, client, &buf, refUpdates)
	ret.PushDebugInfo = &pushDebugInfo
	if err != nil {
		ret.Error = err.Error()
	}
	return ret
}
