// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
)

func Backport(args nichegit.BackportArgs) nichegit.BackportOutput {
	client := &http.Client{Transport: &authnRoundtripper{}}
	return nichegit.Backport(client, args)
}
