// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package debug

type FetchDebugInfo struct {
	// ResponseHeaders is a map of response headers.
	ResponseHeaders map[string][]string `json:"responseHeaders"`
	// PackfileSize is the size of the packfile in bytes.
	PackfileSize int `json:"packfileSize"`
}

type LsRefsDebugInfo struct {
	// ResponseHeaders is the headers of the HTTP response when fetching the packfile.
	ResponseHeaders map[string][]string `json:"responseHeaders"`
}
