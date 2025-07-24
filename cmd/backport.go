// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
)

func Backport(ctx context.Context, args nichegit.BackportArgs) nichegit.BackportOutput {
	client := &http.Client{Transport: &authnRoundtripper{}}
	return nichegit.Backport(ctx, client, args)
}
