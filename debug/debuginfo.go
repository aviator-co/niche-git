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

type PushCommandStatus struct {
	// Name is the name of the reference.
	Name string `json:"name"`
	// Status is the status of the command.
	Status string `json:"status"`
}

type PushDebugInfo struct {
	// PackfileSize is the size of the sent packfile in bytes.
	PackfileSize int `json:"packfileSize"`

	// RefAdvHeaders is the headers of the HTTP response in calling /info/refs
	RefAdvResponseHeaders map[string][]string `json:"refAdvResponseHeaders"`
	// PushResponseHeaders is the headers of the HTTP response in calling /git-receive-pack
	PushResponseHeaders map[string][]string `json:"pushResponseHeaders"`

	// UnpackStatus is the status sent from the server for unpacking the packfile.
	UnpackStatus string `json:"unpackStatus"`
	// CommandStatuses is the status of each command sent to the server.
	CommandStatuses []*PushCommandStatus `json:"commandStatuses"`
}
