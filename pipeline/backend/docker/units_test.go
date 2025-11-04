// Copyright 2025 Crow Authors
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

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Empty and zero
		{"empty string", "", 0, false},
		{"zero", "0", 0, false},

		// Plain bytes
		{"plain bytes", "1024", 1024, false},
		{"plain bytes with spaces", "  2048  ", 2048, false},

		// Kilobytes
		{"KB decimal", "100K", 100000, false},
		{"KB binary", "100Ki", 102400, false},
		{"KB with B", "100KB", 100000, false},
		{"KiB", "100KiB", 102400, false},

		// Megabytes
		{"MB decimal", "512M", 512000000, false},
		{"MB binary", "512Mi", 536870912, false},
		{"MB with B", "512MB", 512000000, false},
		{"MiB", "512MiB", 536870912, false},

		// Gigabytes
		{"GB decimal", "2G", 2000000000, false},
		{"GB binary", "2Gi", 2147483648, false},
		{"GB with B", "2GB", 2000000000, false},
		{"GiB", "2GiB", 2147483648, false},

		// Terabytes
		{"TB decimal", "1T", 1000000000000, false},
		{"TB binary", "1Ti", 1099511627776, false},

		// Decimal values
		{"decimal GB", "1.5G", 1500000000, false},
		{"decimal Gi", "1.5Gi", 1610612736, false},

		// Error cases
		{"invalid format", "2X", 0, true},
		{"invalid unit", "2Zi", 0, true},
		{"invalid number", "abc", 0, true},
		{"negative", "-1G", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMemory(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseCPUQuota(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Empty and zero
		{"empty string", "", 0, false},
		{"zero", "0", 0, false},

		// Cores
		{"1 core", "1", 100000, false},
		{"2 cores", "2", 200000, false},
		{"half core", "0.5", 50000, false},
		{"quarter core", "0.25", 25000, false},

		// Millicores
		{"1000 millicores", "1000m", 100000, false},
		{"500 millicores", "500m", 50000, false},
		{"250 millicores", "250m", 25000, false},
		{"100 millicores", "100m", 10000, false},

		// With spaces
		{"cores with spaces", "  2  ", 200000, false},
		{"millicores with spaces", "  1000m  ", 100000, false},

		// Error cases
		{"invalid format", "2x", 0, true},
		{"invalid unit", "2k", 0, true},
		{"invalid number", "abc", 0, true},
		{"negative", "-1", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCPUQuota(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
