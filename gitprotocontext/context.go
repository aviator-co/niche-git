// Copyright 2024 Aviator Technologies, Inc.
// SPDX-License-Identifier: MIT

package gitprotocontext

import (
	"context"
	"time"
)

type contextKey string

func (c contextKey) String() string {
	return "nichegit context key " + string(c)
}

var (
	fetchTimeoutKey = contextKey("fetchTimeout")
	pushTimeoutKey  = contextKey("pushTimeout")
	fetchRetryKey   = contextKey("fetchRetryCount")
)

func WithGitFetchTimeout(ctx context.Context, timeout time.Duration) context.Context {
	if timeout <= 0 {
		return ctx
	}
	return context.WithValue(ctx, fetchTimeoutKey, timeout)
}

func WithGitFetchRetryCount(ctx context.Context, retryCount int) context.Context {
	if retryCount < 0 {
		return ctx
	}
	return context.WithValue(ctx, fetchRetryKey, retryCount)
}

func WithGitPushTimeout(ctx context.Context, timeout time.Duration) context.Context {
	if timeout <= 0 {
		return ctx
	}
	return context.WithValue(ctx, pushTimeoutKey, timeout)
}

func GitFetchTimeout(ctx context.Context) time.Duration {
	if timeout, ok := ctx.Value(fetchTimeoutKey).(time.Duration); ok && timeout > 0 {
		return timeout
	}
	return 0
}

func GitPushTimeout(ctx context.Context) time.Duration {
	if timeout, ok := ctx.Value(pushTimeoutKey).(time.Duration); ok && timeout > 0 {
		return timeout
	}
	return 0
}

func GitFetchRetryCount(ctx context.Context) int {
	if retryCount, ok := ctx.Value(fetchRetryKey).(int); ok && retryCount >= 0 {
		return retryCount
	}
	return 0
}
