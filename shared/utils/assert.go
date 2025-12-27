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

package utils

import (
	"fmt"
	"time"
)

// used to check if project settings value for keepMin is valid.
// Valid values: -1 (keep all), 0 (use server default), or >0 (specific count).
func IsValidKeepMinValue(value int64) (bool, error) {
	if value >= 0 || value == -1 {
		return true, nil
	}
	return false, fmt.Errorf("invalid keepMin value: %d", value)
}

// used to check if project settings value for keepDuration is valid.
func ValidateDuration(durationStr string) error {
	if durationStr == "" {
		return nil
	}
	_, err := time.ParseDuration(durationStr)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	return nil
}
