// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server"
	"codefloe.com/crowci/crow/v6/server/theme"
)

func TestConfigCustomThemes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	render := func() string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/web-config.js", nil)
		Config(c)
		assert.Equal(t, http.StatusOK, w.Code)
		return w.Body.String()
	}

	original := server.Config.WebUI.CustomThemes
	t.Cleanup(func() { server.Config.WebUI.CustomThemes = original })

	server.Config.WebUI.CustomThemes = nil
	assert.Contains(t, render(), "window.CROW_CUSTOM_THEMES = null;")

	server.Config.WebUI.CustomThemes = []theme.Theme{{
		ID:    "acme",
		Name:  "</script>",
		Light: map[string]string{"primary": "#7c3aed"},
	}}
	assert.Contains(t, render(),
		`window.CROW_CUSTOM_THEMES = [{"id":"acme","name":"\u003c/script\u003e","light":{"primary":"#7c3aed"}}];`)
}
