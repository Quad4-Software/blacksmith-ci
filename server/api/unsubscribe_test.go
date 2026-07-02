// Copyright 2026 Crow CI Authors
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

package api

import (
	"fmt"
	"html"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUnsubscribePage_EscapesRepoName verifies that a repo name carrying HTML
// markup (ultimately user-controlled via the signed token claim) is escaped
// before being rendered into the unsubscribe confirmation page, mirroring the
// message construction in GetUnsubscribe.
func TestUnsubscribePage_EscapesRepoName(t *testing.T) {
	repoName := `crowci/<script>alert('xss')</script>`

	msg := fmt.Sprintf("You have been unsubscribed from email notifications for <strong>%s</strong>.", html.EscapeString(repoName))

	rec := httptest.NewRecorder()
	writeUnsubscribePage(rec, msg, true)

	body := rec.Body.String()

	// Injected markup must not survive verbatim.
	assert.NotContains(t, body, "<script>")
	// The escaped form must be present, while the intentional <strong> wrapper
	// from the handler stays intact.
	assert.Contains(t, body, "crowci/&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;")
	assert.Contains(t, body, "<strong>")
}
