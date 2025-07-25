// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package nichegit

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/internal/fetch"
)

type RefInfo struct {
	// Name is the name of the ref.
	Name string `json:"name"`

	// Hash is the hash of the object that the ref points to.
	//
	// This can be "unborn" if the ref is not created. See man 5 gitprotocol-v2.
	Hash string `json:"hash"`

	// PeeledHash is the hash of the object that the ref points to, if the ref is a tag.
	PeeledHash string `json:"peeledHash,omitempty"`

	// SymbolicTarget is the target of the symbolic ref, if the ref is symbolic.
	SymbolicTarget string `json:"symbolicTarget,omitempty"`
}

type LsRefsArgs struct {
	RepoURL     string   `json:"repoURL"`
	RefPrefixes []string `json:"refPrefixes"`
}

type LsRefsOutput struct {
	Refs      []*RefInfo            `json:"refs"`
	DebugInfo debug.LsRefsDebugInfo `json:"debugInfo"`
	Error     string                `json:"error,omitempty"`
}

func LsRefs(ctx context.Context, client *http.Client, args LsRefsArgs) LsRefsOutput {
	refs, debugInfo, fetchErr := lsRefs(ctx, args.RepoURL, client, args.RefPrefixes)
	if refs == nil {
		// Always create an empty slice for JSON output.
		refs = []*RefInfo{}
	}
	output := LsRefsOutput{
		Refs:      refs,
		DebugInfo: debugInfo,
	}
	if fetchErr != nil {
		output.Error = fetchErr.Error()
	}
	return output
}

func lsRefs(ctx context.Context, repoURL string, client *http.Client, refPrefixes []string) ([]*RefInfo, debug.LsRefsDebugInfo, error) {
	rawRefData, headers, err := fetch.LsRefs(ctx, repoURL, client, refPrefixes)
	debugInfo := debug.LsRefsDebugInfo{ResponseHeaders: headers}
	if err != nil {
		return nil, debugInfo, err
	}
	var refs []*RefInfo
	for _, line := range rawRefData {
		line = strings.TrimSpace(line)
		parts := strings.Split(line, " ")
		if len(parts) < 2 {
			return nil, debugInfo, errors.New("invalid ref line: " + line)
		}
		info := &RefInfo{
			Name: parts[1],
			Hash: parts[0],
		}
		if len(parts) == 3 {
			p := parts[2]
			switch {
			case strings.HasPrefix(p, "symref-target:"):
				info.SymbolicTarget = strings.TrimPrefix(p, "symref-target:")
			case strings.HasPrefix(p, "peeled:"):
				info.PeeledHash = strings.TrimPrefix(p, "peeled:")
			default:
				return nil, debugInfo, errors.New("invalid ref line: " + line)
			}
		}
		refs = append(refs, info)
	}
	return refs, debugInfo, nil
}
