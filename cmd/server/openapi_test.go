package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/crowci/crow/v3/cmd/server/openapi"
)

func TestSetupOpenApiStaticConfig(t *testing.T) {
	setupOpenAPIStaticConfig()
	assert.Equal(t, "/api", openapi.SwaggerInfo.BasePath)
}
