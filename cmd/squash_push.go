// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
)

func SquashPush(args nichegit.SquashPushArgs) nichegit.SquashPushOutput {
	client := &http.Client{Transport: &authnRoundtripper{}}
	return nichegit.SquashPush(client, args)
}
