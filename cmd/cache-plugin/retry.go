// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// retryDelays are the waits between attempts of a transfer or API call. Object
// stores fail a small share of requests transiently, for example a freshly
// created bucket that has not reached every backend yet.
var retryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// statusError is a response with an unexpected HTTP status.
type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string { return e.msg }

// readError is a response body that broke off, for example on a reset
// connection.
type readError struct{ err error }

func (e *readError) Error() string { return e.err.Error() }
func (e *readError) Unwrap() error { return e.err }

// bodyReader tags read errors of a response body as readError.
type bodyReader struct{ io.ReadCloser }

func (b bodyReader) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		err = &readError{err}
	}
	return n, err
}

// retryable reports whether another attempt may succeed: failed requests,
// broken-off bodies, throttling and server errors. A 404 is retried too when
// notFoundIsTransient is set: storage URLs point to an object the server knows
// exists, so a 404 there means the store has not caught up. Everything else,
// such as a cache miss or an archive entry refused on extraction, is final.
func retryable(err error, notFoundIsTransient bool) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var status *statusError
	if errors.As(err, &status) {
		switch {
		case status.code == http.StatusTooManyRequests, status.code >= http.StatusInternalServerError:
			return true
		case status.code == http.StatusNotFound:
			return notFoundIsTransient
		default:
			return false
		}
	}
	var urlErr *url.Error
	var readErr *readError
	return errors.As(err, &urlErr) || errors.As(err, &readErr)
}

// withRetry runs fn until it succeeds, fails for good or runs out of attempts.
// Retries are logged to out, so a slow step is explained.
func withRetry(ctx context.Context, out io.Writer, what string, notFoundIsTransient bool, fn func() error) error {
	err := fn()
	for _, delay := range retryDelays {
		if err == nil || !retryable(err, notFoundIsTransient) {
			return err
		}
		fmt.Fprintf(out, "%s failed, retrying in %s: %v\n", what, delay, err)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(delay):
		}
		err = fn()
	}
	return err
}
