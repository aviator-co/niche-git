// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
)

func SquashPush(ctx context.Context, args nichegit.SquashPushArgs) nichegit.SquashPushOutput {
	client := &http.Client{Transport: &authnRoundtripper{}}
	return nichegit.SquashPush(ctx, client, args)
}
