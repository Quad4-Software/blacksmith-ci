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

package ldap

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

const objectGUIDLen = 16

// FormatObjectGUID formats a 16-byte Active Directory objectGUID as the
// canonical GUID string. The first three groups are stored little-endian.
func FormatObjectGUID(raw []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%x-%x",
		[]byte{raw[3], raw[2], raw[1], raw[0]},
		[]byte{raw[5], raw[4]},
		[]byte{raw[7], raw[6]},
		raw[8:10],
		raw[10:16],
	)
}

// subjectValue turns a raw ID attribute value into a stable string subject.
func subjectValue(attr string, raw []byte) string {
	if strings.EqualFold(attr, "objectGUID") && len(raw) == objectGUIDLen {
		return FormatObjectGUID(raw)
	}
	if !utf8.Valid(raw) {
		return hex.EncodeToString(raw)
	}
	return string(raw)
}
