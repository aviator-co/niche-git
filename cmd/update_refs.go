// Copyright 2025 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"net/http"

	nichegit "github.com/aviator-co/niche-git"
)

func UpdateRefs(args nichegit.UpdateRefsArgs) nichegit.UpdateRefsOutput {
	client := &http.Client{Transport: &authnRoundtripper{}}
	return nichegit.UpdateRefs(client, args)
}
