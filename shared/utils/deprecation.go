// Copyright 2026 Crow authors
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

import "github.com/rs/zerolog/log"

// LogAlpineTagDeprecation prints a prominent startup warning about the
// deprecation of the `-alpine` suffix tags for the given image (e.g.
// `crow-server`, `crow-agent`).
//
// Background: as of v5.x the unsuffixed and `-alpine` tags point to the same
// Alpine-based image. The `-alpine` suffix tags are kept only for backwards
// compatibility and will be removed in a future release.
//
// Users who currently pull `<image>:<tag>-alpine` should switch to
// `<image>:<tag>` (same image, fewer keystrokes).
//
// TODO: remove this warning together with the `-alpine` suffix tags.
func LogAlpineTagDeprecation(image string) {
	log.Warn().Msgf(
		"DEPRECATION: the `-alpine` suffix tags for %[1]s are deprecated and "+
			"will be removed in a future release. The unsuffixed and `-alpine` "+
			"tags currently point to the same image. "+
			"If you pull `%[1]s:<tag>-alpine`, switch to `%[1]s:<tag>`.",
		image,
	)
}
