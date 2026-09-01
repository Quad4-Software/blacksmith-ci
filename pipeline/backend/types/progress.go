// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import "context"

type stepProgressKey struct{}

// WithStepProgress returns a context that reports backend preparation milestones.
func WithStepProgress(ctx context.Context, report func(string)) context.Context {
	return context.WithValue(ctx, stepProgressKey{}, report)
}

// ReportStepProgress reports a backend preparation milestone when the caller supplied a reporter.
func ReportStepProgress(ctx context.Context, message string) {
	if report, ok := ctx.Value(stepProgressKey{}).(func(string)); ok {
		report(message)
	}
}
