// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

var (
	authzHeader        string
	basicAuthzUser     string
	basicAuthzPassword string
)

type authnRoundtripper struct{}

func (rt *authnRoundtripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if authzHeader != "" {
		req.Header.Set("Authorization", authzHeader)
	} else if basicAuthzUser != "" && basicAuthzPassword != "" {
		req.SetBasicAuth(basicAuthzUser, basicAuthzPassword)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func writeJSON(outputPath string, v any) error {
	var of io.Writer
	if outputPath == "-" {
		of = os.Stdout
	} else {
		file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		of = file
	}
	enc := json.NewEncoder(of)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return nil
}
