// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
)

func UpdateRefs(ctx context.Context, args nichegit.UpdateRefsArgs) nichegit.UpdateRefsOutput {
	client := &http.Client{Transport: &authnRoundtripper{}}
	return nichegit.UpdateRefs(ctx, client, args)
}
