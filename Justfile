### Variables

GO_PACKAGES := `go list ./... 2>/dev/null | grep -v /vendor/ || echo ""`
TARGETOS := `go env GOOS`
TARGETARCH := `go env GOARCH`
BIN_SUFFIX := if TARGETOS == "windows" { ".exe" } else { "" }
DIST_DIR := "dist"
# VERSION := env_var_or_default("VERSION", "dev")
CI_COMMIT_SHA := env_var_or_default("CI_COMMIT_SHA", `git rev-parse HEAD 2>/dev/null || echo "unknown"`)
CI_COMMIT_TAG := env_var_or_default("CI_COMMIT_TAG", "")
TAGS := ""
STATIC_BUILD := "true"
CI := env_var_or_default("CI", "false")
BUILD_DATE := `date -u +%Y-%m-%d`
# renovate: datasource=github-releases depName=protocolbuffers/protobuf versioning=loose extractVersion=^v(?<version>.+)$
PROTOC_VERSION := "34.0"
# renovate: datasource=go depName=mvdan.cc/gofumpt
GOFUMPT_VERSION := "v0.9.2"
# renovate: datasource=go depName=github.com/daixiang0/gci
GCI_VERSION := "v0.14.0"
# renovate: datasource=go depName=github.com/vektra/mockery/v3
MOCKERY_VERSION := "v3.7.0"
# renovate: datasource=go depName=github.com/google/addlicense
ADDLICENSE_VERSION := "v1.2.0"
# renovate: datasource=go depName=google.golang.org/protobuf/cmd/protoc-gen-go
PROTOC_GEN_GO_VERSION := "v1.36.11"
# renovate: datasource=go depName=google.golang.org/grpc/cmd/protoc-gen-go-grpc
PROTOC_GEN_GO_GRPC_VERSION := "v1.6.1"

# Set version based on CI environment
VERSION := if env_var_or_default("CI_COMMIT_TAG", "") != "" {
    replace(env_var("CI_COMMIT_TAG"), "v", "")
} else {
    "dev-" + `echo "${CI_COMMIT_SHA:-$(git rev-parse HEAD)}" | cut -c -10 2>/dev/null || echo "unknown"`
}

VERSION_NUMBER := if env_var_or_default("CI_COMMIT_TAG", "") != "" {
    replace(env_var("CI_COMMIT_TAG"), "v", "")
} else {
    env_var_or_default("VERSION_NUMBER", "0.0.0")
}

# Conditional assignment for LDFLAGS if STATIC_BUILD is true
# FIXME: https://github.com/casey/just/issues/11

LDFLAGS := if STATIC_BUILD == "true" {
    "-s -w -extldflags '-static' -X codefloe.com/crowci/crow/v5/version.Version=" + VERSION + " -X codefloe.com/crowci/crow/v5/version.BuildDate=" + BUILD_DATE
} else {
    "-X codefloe.com/crowci/crow/v5/version.Version=" + VERSION + " -X codefloe.com/crowci/crow/v5/version.BuildDate=" + BUILD_DATE
}

# only used to compile server

CGO_ENABLED := "1"
HAS_GO := `hash go > /dev/null 2>&1 && echo "GO" || echo "NOGO"`
XGO_VERSION := if HAS_GO == "GO" { "go-1.26.x" } else { "" }
CGO_CFLAGS := if HAS_GO == "GO" { `go env CGO_CFLAGS` } else { "" }

### Recipes
## general

version:
	@echo "{{VERSION}}"

fmt: install-dev-deps
		prettier -w .
		gofumpt -w .
		golangci-lint run --fix
		gci write --skip-vendor --skip-generated -s standard -s default -s "prefix(codefloe.com/crowci/crow)" --custom-order .

test: test-agent test-server test-server-datastore test-cli test-lib

## Run all code generations
# ensure `protoc-gen-go` and `protoc-gen-go-grpc` are installed
generate: install-dev-deps
	#!/usr/bin/env sh
	set -e
	# Ensure GOPATH/bin is first in PATH to use installed tools
	export PATH="${GOPATH:-$HOME/go}/bin:$PATH"
	# Generate mocks
	mockery
	# Generate protobuf
	cd pipeline/rpc/proto && go generate
	cd -
	# Normalize imports
	gci write --skip-vendor --skip-generated -s standard -s default -s "prefix(codefloe.com/crowci/crow)" --custom-order .

mockery: install-dev-deps
	mockery

install-dev-deps: install-protoc
		#!/usr/bin/env sh
		set -e
		# Install or upgrade Go tools to pinned versions.
		# Checks the binary's embedded module version via `go version -m`
		# and only runs `go install` when the version doesn't match.
		install_go_tool() {
			mod="$1"   # e.g. mvdan.cc/gofumpt
			ver="$2"   # e.g. v0.9.2
			bin="$3"   # e.g. gofumpt
			bin_path="$(command -v "$bin" 2>/dev/null || true)"
			if [ -n "$bin_path" ] && go version -m "$bin_path" 2>/dev/null | grep -qE "^\s+mod\s+\S+\s+${ver}\s"; then
				return
			fi
			echo "Installing ${bin}@${ver}"
			go install "${mod}@${ver}"
		}
		install_go_tool mvdan.cc/gofumpt                                    {{GOFUMPT_VERSION}}              gofumpt
		install_go_tool github.com/daixiang0/gci                            {{GCI_VERSION}}                  gci
		install_go_tool github.com/vektra/mockery/v3                        {{MOCKERY_VERSION}}              mockery
		install_go_tool github.com/google/addlicense                        {{ADDLICENSE_VERSION}}            addlicense
		install_go_tool google.golang.org/protobuf/cmd/protoc-gen-go        {{PROTOC_GEN_GO_VERSION}}         protoc-gen-go
		install_go_tool google.golang.org/grpc/cmd/protoc-gen-go-grpc       {{PROTOC_GEN_GO_GRPC_VERSION}}    protoc-gen-go-grpc

# Install protoc with pinned version (downloads from GitHub releases)
install-protoc:
		#!/usr/bin/env sh
		set -e
		CURRENT_VERSION=$(protoc --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+' | head -1 || echo "")
		if [ "$CURRENT_VERSION" = "{{PROTOC_VERSION}}" ]; then
			exit 0
		fi
		echo "Installing protoc v{{PROTOC_VERSION}}..."
		OS=$(uname -s | tr '[:upper:]' '[:lower:]')
		ARCH=$(uname -m)
		case "$ARCH" in
			x86_64) ARCH="x86_64" ;;
			aarch64|arm64) ARCH="aarch_64" ;;
			*) echo "Unsupported architecture: $ARCH"; exit 1 ;;
		esac
		case "$OS" in
			linux) OS="linux" ;;
			darwin) OS="osx" ;;
			*) echo "Unsupported OS: $OS"; exit 1 ;;
		esac
		URL="https://github.com/protocolbuffers/protobuf/releases/download/v{{PROTOC_VERSION}}/protoc-{{PROTOC_VERSION}}-${OS}-${ARCH}.zip"
		INSTALL_DIR="${GOPATH:-$HOME/go}/bin"
		mkdir -p "$INSTALL_DIR"
		TMP_DIR=$(mktemp -d)
		curl -fsSL "$URL" -o "$TMP_DIR/protoc.zip"
		unzip -q -o "$TMP_DIR/protoc.zip" -d "$TMP_DIR"
		rm -f "$INSTALL_DIR/protoc"
		cp "$TMP_DIR/bin/protoc" "$INSTALL_DIR/protoc"
		chmod +x "$INSTALL_DIR/protoc"
		rm -rf "$TMP_DIR"
		echo "protoc v{{PROTOC_VERSION}} installed to $INSTALL_DIR/protoc"

## test

test-agent:
		CGO_ENABLED=1 go test -race -cover -coverprofile agent-coverage.out -timeout 60s -tags 'test {{ TAGS }}' codefloe.com/crowci/crow/v5/cmd/agent codefloe.com/crowci/crow/v5/agent/...

test-server:
		CGO_ENABLED=1 go test -race -cover -coverprofile server-coverage.out -timeout 60s -tags 'test {{ TAGS }}' codefloe.com/crowci/crow/v5/cmd/server `go list codefloe.com/crowci/crow/v5/server/... | grep -v '/store'`

test-cli:
		CGO_ENABLED=1 go test -race -cover -coverprofile cli-coverage.out -timeout 60s -tags 'test {{ TAGS }}' codefloe.com/crowci/crow/v5/cmd/cli codefloe.com/crowci/crow/v5/cli/...

test-server-datastore:
		CGO_ENABLED=1 go test -timeout 300s -tags 'test {{ TAGS }}' -run TestMigrate codefloe.com/crowci/crow/v5/server/store/...
		go test -race -timeout 100s -tags 'test {{ TAGS }}' -skip TestMigrate codefloe.com/crowci/crow/v5/server/store/...

test-server-datastore-coverage:
		CGO_ENABLED=1 go test -race -cover -coverprofile datastore-coverage.out -timeout 300s -tags 'test {{ TAGS }}' codefloe.com/crowci/crow/v5/server/store/...

# FIXME: feature of 1.38.0 [working-directory('web')]
test-ui:
		cd web && bun install --frozen-lockfile
		cd web && bun run lint
		cd web && bun run format:check
		cd web && bun run typecheck
		cd web && bun run test

test-lib:
		CGO_ENABLED=1 go test -race -cover -coverprofile coverage.out -timeout 60s -tags 'test {{ TAGS }}' `go list ./... | grep -v '/cmd\|/agent\|/cli\|/server'`

## docs

# Install docs dependencies
docs-install:
	cd docs && bun install

# Run docs development server
docs-dev:
	cd docs && bun run dev

# Build docs for production
docs-build:
	cd docs && bun run build

# Preview production build
docs-preview:
	cd docs && bun run preview

# Lint docs markdown files
docs-lint:
	cd docs && bun x markdownlint-cli2 "src/content/**/*.{md,mdx}"

## git

cherry-pick COMMIT:
		git fetch crow && git cherry-pick {{ COMMIT }}

## build

lint-backend: install-dev-deps
		golangci-lint run

deadcode:
		go run golang.org/x/tools/cmd/deadcode@latest -generated=false -f='{{{{println .Path}}{{{{range .Funcs}}{{{{printf "\t%s\n" .Name}}{{{{end}}{{{{println}}' -test codefloe.com/crowci/crow/v5/...

# FIXME: feature of 1.38.0 [working-directory('web')]
lint-frontend:
		cd web/ && bun run lint --quiet

# FIXME: feature of 1.38.0 [working-directory('web')]
build-ui:
		cd web/ && bun install --frozen-lockfile
		cd web/ && bun run build

build-agent:
		CGO_ENABLED=0 GOOS={{ TARGETOS }} GOARCH={{ TARGETARCH }} go build -tags '{{ TAGS }}' -ldflags '{{ LDFLAGS }}' -o {{ DIST_DIR }}/crow-agent{{ BIN_SUFFIX }} codefloe.com/crowci/crow/v5/cmd/agent

build-cli:
		CGO_ENABLED=0 GOOS={{ TARGETOS }} GOARCH={{ TARGETARCH }} go build -tags '{{ TAGS }}' -ldflags '{{ LDFLAGS }}' -o {{ DIST_DIR }}/crow-cli{{ BIN_SUFFIX }} codefloe.com/crowci/crow/v5/cmd/cli

install-cli-dev: build-cli
		mkdir -p ~/.local/bin
		cp {{ DIST_DIR }}/crow-cli{{ BIN_SUFFIX }} ~/.local/bin/crow-dev{{ BIN_SUFFIX }}
		@echo "Installed crow-dev to ~/.local/bin/crow-dev"

# build-server

# env PLATFORMS=linux/amd64 just build-server
build-server: build-ui vendor
		just cross-compile-server

vendor:
		go mod tidy
		go mod vendor

# build for specific platform: env PLATFORMS='linux|arm64/v8' just cross-compile-server

# build for local platform: just cross-compile-server
cross-compile-server:
		#!/usr/bin/env bash
		set -euxo pipefail

		PLATFORMS="${PLATFORMS:-{{ TARGETOS }}/{{ TARGETARCH }}}"
		IFS=';' read -ra PLATFORMS_ARRAY <<< "${PLATFORMS}"
		for platform in "${PLATFORMS_ARRAY[@]}"; do
				IFS='|' read -ra PLATFORM_PARTS <<< "$platform"
				TARGETOS="${PLATFORM_PARTS[0]}"
				if [ -n "${PLATFORM_PARTS[1]:-}" ]; then
						TARGETARCH_XGO="${PLATFORM_PARTS[1]//arm64\/v8/arm64}"
						TARGETARCH_XGO="${TARGETARCH_XGO//arm\/v7/arm-7}"
						TARGETARCH="${PLATFORM_PARTS[1]//arm64\/v8/arm64}"
						TARGETARCH="${TARGETARCH//arm\/v7/arm}"
				else
						TARGETARCH_XGO=""
						TARGETARCH=""
				fi
				env TARGETOS=$TARGETOS TARGETARCH=$TARGETARCH TARGETARCH_XGO=$TARGETARCH_XGO just release-server-xgo || exit 1
		done
		tree "{{ DIST_DIR }}"

check-xgo:
		hash xgo > /dev/null 2>&1 || go install src.techknowlogick.com/xgo@latest

# this should not be called directly - use 'cross-compile-server' instead
release-server-xgo: check-xgo
		@echo "------------------"
		@echo "Building for:"
		@echo "- os: ${TARGETOS}"
		@echo "- arch (xgo): ${TARGETARCH}"
		@echo "------------------"
		CGO_CFLAGS="{{ CGO_CFLAGS }}" xgo -go {{ XGO_VERSION }} -dest {{ DIST_DIR }}/server/${TARGETOS}_${TARGETARCH} -tags 'netgo osusergo grpcnotrace {{ TAGS }}' -ldflags '-linkmode external {{ LDFLAGS }}' -targets ${TARGETOS}/${TARGETARCH_XGO} -out crow-server -pkg cmd/server .
		# move binary into subfolder depending on target os and arch
		if [ "${XGO_IN_XGO:-0}" -eq "1" ]; then \
			echo "inside xgo image"; \
			mkdir -p {{ DIST_DIR }}/server/${TARGETOS}_${TARGETARCH}; \
			mv -vf /build/crow-server* {{ DIST_DIR }}/server/${TARGETOS}_${TARGETARCH}/crow-server{{ BIN_SUFFIX }}; \
		else \
			echo "outside xgo image"; \
			[ -f "{{ DIST_DIR }}/server/${TARGETOS}_${TARGETARCH}/crow-server{{ BIN_SUFFIX }}" ] && rm -v {{ DIST_DIR }}/server/${TARGETOS}_${TARGETARCH}/crow-server{{ BIN_SUFFIX }}; \
			mv -v {{ DIST_DIR }}/server/${TARGETOS}_${TARGETARCH}/crow-server* {{ DIST_DIR }}/server/${TARGETOS}_${TARGETARCH}/crow-server{{ BIN_SUFFIX }}; \
		fi

# # Create checksums for all release files
release-checksums:
		(cd {{ DIST_DIR }}/; sha256sum *.* > checksums.txt)

release-cli: ## Create cli binaries for release
	# compile
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '{{LDFLAGS}}' -o {{DIST_DIR}}/cli/linux_amd64/crow-cli       codefloe.com/crowci/crow/v5/cmd/cli
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build -ldflags '{{LDFLAGS}}' -o {{DIST_DIR}}/cli/linux_arm64/crow-cli       codefloe.com/crowci/crow/v5/cmd/cli
	GOOS=linux   GOARCH=arm   CGO_ENABLED=0 go build -ldflags '{{LDFLAGS}}' -o {{DIST_DIR}}/cli/linux_arm/crow-cli         codefloe.com/crowci/crow/v5/cmd/cli
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '{{LDFLAGS}}' -o {{DIST_DIR}}/cli/windows_amd64/crow-cli.exe codefloe.com/crowci/crow/v5/cmd/cli
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '{{LDFLAGS}}' -o {{DIST_DIR}}/cli/darwin_amd64/crow-cli      codefloe.com/crowci/crow/v5/cmd/cli
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -ldflags '{{LDFLAGS}}' -o {{DIST_DIR}}/cli/darwin_arm64/crow-cli      codefloe.com/crowci/crow/v5/cmd/cli
	# tar binary files
	tar -cvzf {{DIST_DIR}}/crow-cli_linux_amd64.tar.gz   -C {{DIST_DIR}}/cli/linux_amd64   crow-cli
	tar -cvzf {{DIST_DIR}}/crow-cli_linux_arm64.tar.gz   -C {{DIST_DIR}}/cli/linux_arm64   crow-cli
	tar -cvzf {{DIST_DIR}}/crow-cli_linux_arm.tar.gz     -C {{DIST_DIR}}/cli/linux_arm     crow-cli
	tar -cvzf {{DIST_DIR}}/crow-cli_darwin_amd64.tar.gz  -C {{DIST_DIR}}/cli/darwin_amd64  crow-cli
	tar -cvzf {{DIST_DIR}}/crow-cli_darwin_arm64.tar.gz  -C {{DIST_DIR}}/cli/darwin_arm64  crow-cli
	# zip binary files


# Build agent for all platforms: just release-agent
# Build for specific platforms: env PLATFORMS='darwin' just release-agent
# Build for multiple platforms: env PLATFORMS='darwin linux' just release-agent
release-agent: ## Create agent binaries for release
	#!/usr/bin/env bash
	set -euo pipefail

	PLATFORMS="${PLATFORMS:-linux darwin windows}"

	declare -A os_archs=(
		["linux"]="amd64 arm64 arm"
		["darwin"]="amd64 arm64"
		["windows"]="amd64"
	)

	for os in $PLATFORMS; do
		if [[ ! -v os_archs[$os] ]]; then
			echo "Unknown OS: $os"
			exit 1
		fi

		for arch in ${os_archs[$os]}; do
			bin_suffix=""
			[[ "$os" == "windows" ]] && bin_suffix=".exe"

			echo "Building crow-agent for $os/$arch..."
			mkdir -p {{DIST_DIR}}/agent/${os}_${arch}
			GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -ldflags '{{LDFLAGS}}' \
				-o {{DIST_DIR}}/agent/${os}_${arch}/crow-agent${bin_suffix} \
				codefloe.com/crowci/crow/v5/cmd/agent

			# Create archives
			if [[ "$os" == "windows" ]]; then
				rm -f {{DIST_DIR}}/crow-agent_${os}_${arch}.zip
				(cd {{DIST_DIR}}/agent/${os}_${arch} && zip ../../crow-agent_${os}_${arch}.zip crow-agent${bin_suffix})
			else
				tar -czf {{DIST_DIR}}/crow-agent_${os}_${arch}.tar.gz \
					-C {{DIST_DIR}}/agent/${os}_${arch} crow-agent
			fi
		done
	done

	echo "Build complete. Artifacts in {{DIST_DIR}}/"
	ls -lh {{DIST_DIR}}/crow-agent_*

## images
# platforms must be handed over via this syntax for the underlying cross-compile-server step which applies some string splitting on a list of items

# env PLATFORMS='linux|amd64;linux|arm64' just image-server
image-server TAG REGISTRY_USER='crowci-bot' REGISTRY='codefloe.com' IMAGE='crowci/crow-server':
	just build-ui
	just cross-compile-server
	echo $crowci_bot_token | docker login {{REGISTRY}} -u {{REGISTRY_USER}} --password-stdin
	FIXED_PLATFORMS=$(echo $PLATFORMS | sed "s/|/\//g; s/;/,/g") && \
	docker buildx build --platform $FIXED_PLATFORMS \
		--build-arg CI_COMMIT_SHA={{CI_COMMIT_SHA}} \
		--build-arg CI_COMMIT_TAG={{CI_COMMIT_TAG}} \
		-t {{REGISTRY}}/{{IMAGE}}:{{TAG}} \
		-f images/Containerfile.server.multiarch.rootless --push .

# env PLATFORMS='linux|amd64;linux|arm64' just image-server-alpine dev
image-server-alpine TAG REGISTRY_USER='crowci-bot' REGISTRY='codefloe.com' IMAGE='crowci/crow-server':
	just build-ui
	just cross-compile-server
	echo $crowci_bot_token | docker login {{REGISTRY}} -u {{REGISTRY_USER}} --password-stdin
	FIXED_PLATFORMS=$(echo $PLATFORMS | sed "s/|/\//g; s/;/,/g") && \
	docker buildx build --platform $FIXED_PLATFORMS \
		--build-arg CACHE_BUST=$(date +%s) \
		--build-arg CI_COMMIT_SHA={{CI_COMMIT_SHA}} \
		--build-arg CI_COMMIT_TAG={{CI_COMMIT_TAG}} \
		-t {{REGISTRY}}/{{IMAGE}}:{{TAG}}-alpine \
		-f images/Containerfile.server.alpine.multiarch.rootless --push .

# env PLATFORMS='linux/amd64,linux/arm64' just image-agent
image-agent TAG REGISTRY_USER='crowci-bot' REGISTRY='codefloe.com' IMAGE='crowci/crow-agent':
	echo $crowci_bot_token | docker login {{REGISTRY}} -u {{REGISTRY_USER}} --password-stdin
	docker buildx build --platform $PLATFORMS \
		--build-arg CI_COMMIT_SHA={{CI_COMMIT_SHA}} \
		--build-arg CI_COMMIT_TAG={{CI_COMMIT_TAG}} \
		-t {{REGISTRY}}/{{IMAGE}}:{{TAG}} \
		-f images/Containerfile.agent.multiarch --push .

# env PLATFORMS='linux/amd64,linux/arm64' just image-agent-alpine dev
image-agent-alpine TAG REGISTRY_USER='crowci-bot' REGISTRY='codefloe.com' IMAGE='crowci/crow-agent':
	echo $crowci_bot_token | docker login {{REGISTRY}} -u {{REGISTRY_USER}} --password-stdin
	docker buildx build --platform $PLATFORMS \
		--build-arg CI_COMMIT_SHA={{CI_COMMIT_SHA}} \
		--build-arg CI_COMMIT_TAG={{CI_COMMIT_TAG}} \
		-t {{REGISTRY}}/{{IMAGE}}:{{TAG}}-alpine \
		-f images/Containerfile.agent.alpine.multiarch --push .

# env PLATFORMS='linux/amd64,linux/arm64' just image-cli dev
image-cli TAG REGISTRY_USER='crowci-bot' REGISTRY='codefloe.com' IMAGE='crowci/crow-cli':
	echo $crowci_bot_token | docker login {{REGISTRY}} -u {{REGISTRY_USER}} --password-stdin
	docker buildx build --platform $PLATFORMS \
		--build-arg CI_COMMIT_SHA={{CI_COMMIT_SHA}} \
		--build-arg CI_COMMIT_TAG={{CI_COMMIT_TAG}} \
		-t {{REGISTRY}}/{{IMAGE}}:{{TAG}} \
		-f images/Containerfile.cli.multiarch.rootless --push .

# env PLATFORMS='linux/amd64,linux/arm64' just image-cli-alpine dev
image-cli-alpine TAG REGISTRY_USER='crowci-bot' REGISTRY='codefloe.com' IMAGE='crowci/crow-cli':
	echo $crowci_bot_token | docker login {{REGISTRY}} -u {{REGISTRY_USER}} --password-stdin
	docker buildx build --platform $PLATFORMS \
		--build-arg CI_COMMIT_SHA={{CI_COMMIT_SHA}} \
		--build-arg CI_COMMIT_TAG={{CI_COMMIT_TAG}} \
		-t {{REGISTRY}}/{{IMAGE}}:{{TAG}}-alpine \
		-f images/Containerfile.cli.alpine.multiarch.rootless --push .
