// Copyright 2023 Woodpecker Authors
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

package utils

import (
	"regexp"
	"strings"

	"github.com/distribution/reference"
	version "github.com/hashicorp/go-version"
)

// MajorMinorVersionSegments represents the number of segments for major.minor version format.
const majorMinorVersionSegments = 2

// trimImage returns the short image name without tag.
func trimImage(name string) string {
	ref, err := reference.ParseAnyReference(name)
	if err != nil {
		return name
	}
	named, err := reference.ParseNamed(ref.String())
	if err != nil {
		return name
	}
	named = reference.TrimNamed(named)
	return reference.FamiliarName(named)
}

// expandImage returns the fully qualified image name.
func expandImage(name string) string {
	ref, err := reference.ParseAnyReference(name)
	if err != nil {
		return name
	}
	named, err := reference.ParseNamed(ref.String())
	if err != nil {
		return name
	}
	named = reference.TagNameOnly(named)
	return named.String()
}

// MatchImage returns true if the image name matches
// an image in the list. Note the image tag is not used
// in the matching logic.
func MatchImage(from string, to ...string) bool {
	from = trimImage(from)
	for _, match := range to {
		if from == trimImage(match) {
			return true
		}
	}
	return false
}

// MatchImageDynamic check if image is in list based on list.
// If an list entry has a tag specified it only will match if both are the same, else the tag is ignored.
func MatchImageDynamic(from string, to ...string) bool {
	fullFrom := expandImage(from)
	trimFrom := trimImage(from)
	for _, match := range to {
		if imageHasTag(match) {
			if fullFrom == expandImage(match) {
				return true
			}
		} else {
			if trimFrom == trimImage(match) {
				return true
			}
		}
	}
	return false
}

func imageHasTag(name string) bool {
	return strings.Contains(name, ":")
}

// ParseNamed parses an image as a reference to validate it then parses it as a named reference.
func ParseNamed(image string) (reference.Named, error) {
	ref, err := reference.ParseAnyReference(image)
	if err != nil {
		return nil, err
	}
	return reference.ParseNamed(ref.String())
}

// MatchHostname returns true if the image hostname
// matches the specified hostname.
func MatchHostname(image, hostname string) bool {
	named, err := ParseNamed(image)
	if err != nil {
		return false
	}
	if hostname == "index.docker.io" {
		hostname = "docker.io"
	}
	return reference.Domain(named) == hostname
}

// MatchImageWithType matches an image against a list using the specified match type: semver, semver-range, regex, or exact.
func MatchImageWithType(from, matchType string, to ...string) bool {
	switch matchType {
	case "regex":
		for _, pattern := range to {
			matched, err := regexp.MatchString(pattern, from)
			if err == nil && matched {
				return true
			}
		}
		return false
	case "exact":
		for _, pattern := range to {
			if from == pattern {
				return true
			}
		}
		return false
	case "semver":
		return matchImageSemver(from, to...)
	case "semver-range":
		return matchImageSemverRange(from, to...)
	default:
		return matchImageSemver(from, to...)
	}
}

// getImageTag extracts the tag from an image string, or returns an empty string if none.
func getImageTag(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// matchImageSemver matches image tags using semver logic (major, major.minor, or full version).
func matchImageSemver(from string, to ...string) bool {
	fromRef, err := reference.ParseAnyReference(from)
	if err != nil {
		return false
	}
	fromNamed, err := reference.ParseNamed(fromRef.String())
	if err != nil {
		return false
	}
	fromNamed = reference.TrimNamed(fromNamed)
	fromName := reference.FamiliarName(fromNamed)
	var fromTag string
	if tagged, ok := fromNamed.(reference.Tagged); ok {
		fromTag = tagged.Tag()
	} else {
		fromTag = getImageTag(from)
	}
	for _, pattern := range to {
		patternRef, err := reference.ParseAnyReference(pattern)
		if err != nil {
			continue
		}
		patternNamed, err := reference.ParseNamed(patternRef.String())
		if err != nil {
			continue
		}
		patternNamed = reference.TrimNamed(patternNamed)
		patternName := reference.FamiliarName(patternNamed)
		var patternTag string
		if tagged, ok := patternNamed.(reference.Tagged); ok {
			patternTag = tagged.Tag()
		} else {
			patternTag = getImageTag(pattern)
		}
		if fromName != patternName {
			continue
		}
		if patternTag == "" {
			if !strings.Contains(pattern, ":") {
				return true
			}
			continue
		}
		if fromTag == patternTag {
			return true
		}
		fromVer, err1 := version.NewVersion(fromTag)
		patternVer, err2 := version.NewVersion(patternTag)
		if err1 == nil && err2 == nil {
			if len(strings.Split(patternTag, ".")) == 1 {
				if fromVer.Segments()[0] == patternVer.Segments()[0] {
					return true
				}
			}
			if len(strings.Split(patternTag, ".")) == majorMinorVersionSegments {
				if len(fromVer.Segments()) >= majorMinorVersionSegments && fromVer.Segments()[0] == patternVer.Segments()[0] && fromVer.Segments()[1] == patternVer.Segments()[1] {
					return true
				}
			}
			if patternTag == fromTag {
				return true
			}
		}
	}
	return false
}

// matchImageSemverRange matches image tags using semver range constraints (e.g., >=6, <7.0, ~6.3).
func matchImageSemverRange(from string, to ...string) bool {
	fromRef, err := reference.ParseAnyReference(from)
	if err != nil {
		return false
	}
	fromNamed, err := reference.ParseNamed(fromRef.String())
	if err != nil {
		return false
	}
	fromNamed = reference.TrimNamed(fromNamed)
	fromName := reference.FamiliarName(fromNamed)
	var fromTag string
	if tagged, ok := fromNamed.(reference.Tagged); ok {
		fromTag = tagged.Tag()
	} else {
		fromTag = getImageTag(from)
	}
	for _, pattern := range to {
		// For semver-range, we need to parse the pattern differently
		// Pattern format: "image:constraint" where constraint is a semver constraint
		const expectedParts = 2
		parts := strings.SplitN(pattern, ":", expectedParts)
		if len(parts) != expectedParts {
			continue
		}
		patternName := parts[0]
		constraintStr := parts[1]

		if fromName != patternName {
			continue
		}

		// Try semver range match
		fromVer, err1 := version.NewVersion(fromTag)
		if err1 != nil {
			continue
		}
		constraints, err2 := version.NewConstraint(constraintStr)
		if err2 != nil {
			continue
		}
		if constraints.Check(fromVer) {
			return true
		}
	}
	return false
}
