// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package push

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/aviator-co/niche-git/debug"
	"github.com/aviator-co/niche-git/gitprotocontext"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	gogittransport "github.com/go-git/go-git/v5/plumbing/transport"
	gogitfile "github.com/go-git/go-git/v5/plumbing/transport/file"
	gogithttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

func Push(ctx context.Context, repoURL string, client *http.Client, packfile *bytes.Buffer, refUpdates []RefUpdate) (debug.PushDebugInfo, error) {
	debugInfo := debug.PushDebugInfo{}
	if packfile != nil {
		debugInfo.PackfileSize = packfile.Len()
	}
	if client == nil {
		client = http.DefaultClient
	}

	crt := &capturingRoundTripper{inner: client.Transport}
	var transport gogittransport.Transport
	var isFile bool
	if strings.HasPrefix(repoURL, "file") {
		repoURL = strings.TrimPrefix(repoURL, "file://")
		isFile = true
	}

	ep, err := gogittransport.NewEndpoint(repoURL)
	if err != nil {
		return debugInfo, err
	}

	if isFile {
		transport = gogitfile.DefaultClient
	} else {
		transport = gogithttp.NewClient(&http.Client{
			Transport:     crt,
			CheckRedirect: client.CheckRedirect,
			Jar:           client.Jar,
			Timeout:       client.Timeout,
		})
	}

	sess, err := transport.NewReceivePackSession(ep, nil)
	if err != nil {
		return debugInfo, err
	}
	defer sess.Close()

	advRef, err := sess.AdvertisedReferences()
	debugInfo.RefAdvResponseHeaders = crt.lastResponseHTTPHeader
	if err != nil {
		return debugInfo, err
	}

	req := packp.NewReferenceUpdateRequestFromCapabilities(advRef.Capabilities)
	_ = req.Capabilities.Add("atomic")
	if packfile != nil {
		req.Packfile = io.NopCloser(packfile)
	}
	for _, u := range refUpdates {
		cmd := &packp.Command{
			Name: u.Name,
			New:  u.NewHash,
		}
		if u.OldHash != nil {
			cmd.Old = *u.OldHash
		} else if h, ok := advRef.References[u.Name.String()]; ok {
			cmd.Old = h
		} else {
			cmd.Old = plumbing.ZeroHash
		}
		req.Commands = append(req.Commands, cmd)
	}
	ctx, cancel := ctx, func() {}
	if timeout := gitprotocontext.GitPushTimeout(ctx); timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	status, err := sess.ReceivePack(ctx, req)
	debugInfo.PushResponseHeaders = crt.lastResponseHTTPHeader
	if status != nil {
		debugInfo.UnpackStatus = status.UnpackStatus
		for _, cs := range status.CommandStatuses {
			debugInfo.CommandStatuses = append(debugInfo.CommandStatuses, &debug.PushCommandStatus{
				Name:   cs.ReferenceName.String(),
				Status: cs.Status,
			})
		}
	}
	if err != nil {
		return debugInfo, err
	}
	if status != nil {
		if err := status.Error(); err != nil {
			return debugInfo, err
		}
	}
	return debugInfo, nil
}

type RefUpdate struct {
	Name plumbing.ReferenceName

	// OldHash, if set, is the expected value of the reference. If the value is not set, the
	// reference will be updated to the new value unconditionally. Use ZeroHash if you expect
	// the reference to not exist.
	OldHash *plumbing.Hash
	// NewHash is the value that the reference will be updated to.
	NewHash plumbing.Hash
}

type capturingRoundTripper struct {
	inner                  http.RoundTripper
	lastResponseHTTPHeader http.Header
}

func (crt *capturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if crt.inner == nil {
		crt.inner = http.DefaultTransport
	}
	resp, err := crt.inner.RoundTrip(req)
	if resp != nil {
		crt.lastResponseHTTPHeader = resp.Header.Clone()
	}
	return resp, err
}
