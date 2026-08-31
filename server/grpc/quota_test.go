// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotaClass(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		want     string
	}{
		{name: "linux architecture", platform: "linux/amd64", want: "linux"},
		{name: "windows architecture", platform: "windows/amd64", want: "windows"},
		{name: "Darwin is macOS", platform: "darwin/arm64", want: "macos"},
		{name: "missing platform defaults to Linux", want: "linux"},
		{name: "unknown platform defaults to Linux", platform: "freebsd/amd64", want: "linux"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, quotaClass(test.platform))
		})
	}
}
