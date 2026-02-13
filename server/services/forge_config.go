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

package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v5/server/model"
)

// ForgeConfig represents the configuration for a single forge parsed from environment variables.
type ForgeConfig struct {
	Name              string
	Type              model.ForgeType
	URL               string
	Client            string
	ClientSecret      string
	SkipVerify        bool
	OAuthHost         string
	Icon              string // custom icon name (e.g., "codeberg", "github") or URL to custom icon
	AdditionalOptions map[string]any
}

// ParseMultiForgeConfig parses multi-forge configuration from environment variables.
// It expects CROW_FORGES to contain a comma-separated list of forge names, and then
// for each forge name, it reads CROW_FORGE_{NAME}_TYPE, CROW_FORGE_{NAME}_URL, etc.
//
// Example:
//
//	CROW_FORGES=codeberg,github
//	CROW_FORGE_CODEBERG_TYPE=forgejo
//	CROW_FORGE_CODEBERG_URL=https://codeberg.org
//	CROW_FORGE_CODEBERG_CLIENT=xxx
//	CROW_FORGE_CODEBERG_SECRET=yyy
//	CROW_FORGE_GITHUB_TYPE=github
//	CROW_FORGE_GITHUB_URL=https://github.com
//	CROW_FORGE_GITHUB_CLIENT=aaa
//	CROW_FORGE_GITHUB_SECRET=bbb
func ParseMultiForgeConfig() ([]ForgeConfig, error) {
	forgesEnv := os.Getenv("CROW_FORGES")
	if forgesEnv == "" {
		return nil, nil // No multi-forge config
	}

	forgeNames := strings.Split(forgesEnv, ",")
	var configs []ForgeConfig

	for _, name := range forgeNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		config, err := parseForgeEnvVars(name)
		if err != nil {
			return nil, fmt.Errorf("failed to parse forge %q: %w", name, err)
		}

		configs = append(configs, config)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("CROW_FORGES is set but no valid forge configurations found")
	}

	return configs, nil
}

// parseForgeEnvVars parses environment variables for a specific forge name.
// The name is converted to uppercase and hyphens are replaced with underscores for env var lookup.
func parseForgeEnvVars(name string) (ForgeConfig, error) {
	prefix := "CROW_FORGE_" + normalizeEnvVarName(name) + "_"

	config := ForgeConfig{
		Name:              name,
		AdditionalOptions: make(map[string]any),
	}

	// Parse type (required)
	typeStr := getEnvOrFile(prefix+"TYPE", prefix+"TYPE_FILE")
	if typeStr == "" {
		return config, fmt.Errorf("missing %sTYPE", prefix)
	}

	forgeType, err := parseForgeType(typeStr)
	if err != nil {
		return config, fmt.Errorf("invalid %sTYPE: %w", prefix, err)
	}
	config.Type = forgeType

	// Parse URL (may have default based on type)
	config.URL = getEnvOrFile(prefix+"URL", prefix+"URL_FILE")
	if config.URL == "" {
		config.URL = getDefaultURL(forgeType)
	}

	// Parse OAuth credentials (required for most forges)
	config.Client = strings.TrimSpace(getEnvOrFile(prefix+"CLIENT", prefix+"CLIENT_FILE"))
	config.ClientSecret = strings.TrimSpace(getEnvOrFile(prefix+"SECRET", prefix+"SECRET_FILE"))

	// Parse optional settings
	config.SkipVerify = getEnvBool(prefix + "SKIP_VERIFY")
	config.OAuthHost = getEnvOrFile(prefix+"OAUTH_HOST", prefix+"OAUTH_HOST_FILE")
	config.Icon = getEnvOrFile(prefix+"ICON", prefix+"ICON_FILE")

	// Parse type-specific options
	switch forgeType {
	case model.ForgeTypeGithub:
		config.AdditionalOptions["merge-ref"] = getEnvBoolDefault(prefix+"MERGE_REF", true)
		config.AdditionalOptions["public-only"] = getEnvBool(prefix + "PUBLIC_ONLY")
	case model.ForgeTypeBitbucketDatacenter:
		config.AdditionalOptions["git-username"] = getEnvOrFile(prefix+"GIT_USERNAME", prefix+"GIT_USERNAME_FILE")
		config.AdditionalOptions["git-password"] = getEnvOrFile(prefix+"GIT_PASSWORD", prefix+"GIT_PASSWORD_FILE")
		config.AdditionalOptions["oauth-enable-project-admin-scope"] = getEnvBool(prefix + "OAUTH_ENABLE_PROJECT_ADMIN_SCOPE")
	case model.ForgeTypeAddon:
		config.AdditionalOptions["executable"] = getEnvOrFile(prefix+"EXECUTABLE", prefix+"EXECUTABLE_FILE")
	}

	log.Debug().
		Str("name", name).
		Str("type", string(forgeType)).
		Str("url", config.URL).
		Bool("skip-verify", config.SkipVerify).
		Bool("client-set", config.Client != "").
		Bool("secret-set", config.ClientSecret != "").
		Msg("parsed forge configuration")

	return config, nil
}

// normalizeEnvVarName converts a forge name to a valid environment variable name component.
// It converts to uppercase and replaces hyphens with underscores.
func normalizeEnvVarName(name string) string {
	name = strings.ToUpper(name)
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

// parseForgeType converts a string to a ForgeType.
func parseForgeType(typeStr string) (model.ForgeType, error) {
	switch strings.ToLower(strings.TrimSpace(typeStr)) {
	case "github":
		return model.ForgeTypeGithub, nil
	case "gitlab":
		return model.ForgeTypeGitlab, nil
	case "gitea":
		return model.ForgeTypeGitea, nil
	case "forgejo":
		return model.ForgeTypeForgejo, nil
	case "bitbucket":
		return model.ForgeTypeBitbucket, nil
	case "bitbucket-dc", "bitbucket-datacenter", "stash":
		return model.ForgeTypeBitbucketDatacenter, nil
	case "addon":
		return model.ForgeTypeAddon, nil
	default:
		return "", fmt.Errorf("unknown forge type: %s", typeStr)
	}
}

// getDefaultURL returns the default URL for a forge type.
func getDefaultURL(forgeType model.ForgeType) string {
	switch forgeType {
	case model.ForgeTypeGithub:
		return "https://github.com"
	case model.ForgeTypeGitlab:
		return "https://gitlab.com"
	case model.ForgeTypeGitea:
		return "https://try.gitea.com"
	case model.ForgeTypeForgejo:
		return "https://next.forgejo.org"
	default:
		return ""
	}
}

// getEnvOrFile gets a value from an environment variable, or reads it from a file
// if the file environment variable is set.
func getEnvOrFile(envVar, fileEnvVar string) string {
	// First check if there's a file to read from
	if filePath := os.Getenv(fileEnvVar); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Warn().Err(err).Str("file", filePath).Msg("failed to read secret from file")
			return ""
		}
		return strings.TrimSpace(string(data))
	}

	return os.Getenv(envVar)
}

// getEnvBool gets a boolean value from an environment variable.
func getEnvBool(envVar string) bool {
	val := strings.ToLower(os.Getenv(envVar))
	return val == "true" || val == "1" || val == "yes"
}

// getEnvBoolDefault gets a boolean value from an environment variable with a default.
func getEnvBoolDefault(envVar string, defaultVal bool) bool {
	val := os.Getenv(envVar)
	if val == "" {
		return defaultVal
	}
	val = strings.ToLower(val)
	return val == "true" || val == "1" || val == "yes"
}
