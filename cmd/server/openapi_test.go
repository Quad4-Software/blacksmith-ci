package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v5/cmd/server/openapi"
)

func TestSetupOpenApiStaticConfig(t *testing.T) {
	setupOpenAPIStaticConfig()
	assert.Equal(t, "/api/v1", openapi.SwaggerInfo.BasePath)
}
