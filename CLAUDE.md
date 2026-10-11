# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Task

feature: implement SMTP auth using a modern go mailing sdk and allow configuring smtp notification on a per pipeline basis

## Development Commands

### Build Commands

- `just build-server` - Build the server binary (includes UI build)
- `just build-agent` - Build the agent binary
- `just build-cli` - Build the CLI binary
- `just build-ui` - Build the Vue.js frontend
- `just vendor` - Update Go module dependencies

### Testing Commands

- `just test` - Run all tests (agent, server, cli, lib)
- `just test-agent` - Run agent tests with coverage
- `just test-server` - Run server tests with coverage
- `just test-cli` - Run CLI tests with coverage
- `just test-ui` - Run frontend tests (lint, format, typecheck, test)
- `just test-server-datastore` - Run database migration and store tests

**Running single tests:**

- `go test -v -run TestName ./path/to/package` - Run specific test
- `go test -tags 'test' -run TestName ./path/to/package` - Run test with build tags
- Example: `go test -v -run TestPipeline codeberg.org/crowci/crow/v6/server/pipeline`

### Linting and Formatting

- `just lint-backend` - Run Go linter (golangci-lint)
- `just lint-frontend` - Run frontend linter (eslint with `--max-warnings 0`)
- `just fmt` - Format code (prettier + gofmt + gci)

**Frontend lint rules (e18e plugin):** CI enforces zero warnings. Key rules to follow:

- Use `[...iterable]` instead of `Array.from(iterable)` (no mapper)
- Put regex literals at module scope, not inside functions
- Use `.toSorted()` instead of `[...arr].sort()`
- Use `.at(-1)` instead of `arr[arr.length - 1]`
- Use `Date.now()` instead of `new Date().getTime()`
- Use `.some()` instead of `.find()` in boolean context

### Code Generation

- `just generate` - Run all code generation
- `just generate-openapi` - Generate OpenAPI documentation
- `just mockery` - Generate mocks for testing

### Development Tools

- `just install-dev-deps` - Install development dependencies
- Frontend commands (run from `/web` directory):
  - `bun install` - Install frontend dependencies
  - `bun run lint` - Lint frontend code
  - `bun run typecheck` - Type check frontend code
  - `bun run test` - Run frontend tests

### Running Locally

- `./dist/crow-server` - Run server (after building with `just build-server`)
- `./dist/crow-agent` - Run agent (after building with `just build-agent`)
- `./dist/crow` - Run CLI (after building with `just build-cli`)
- Environment variables can be set via `.env` file (auto-loaded) or command line
- Use `--help` flag to see all available options for each binary

### Git and Forgejo/Codeberg CLI

Use `fj` (forgejo-cli) for Codeberg/Forgejo operations:

- `fj pr create "title" --body "description"` - Create a pull request
- `fj pr view <number>` - View a pull request
- `fj pr checkout <number>` - Checkout a PR locally
- `fj pr merge <number>` - Merge a pull request
- `fj issue create "title" --body "description"` - Create an issue
- `fj issue view <number>` - View an issue
- `fj auth login` - Authenticate with Codeberg
- `fj whoami` - Check current authentication status

Note: Use `-R origin` flag with `fj pr` commands to specify the remote if needed.

## Project Architecture

### Core Components

**Server** (`/cmd/server/`, `/server/`)

- Central orchestration hub and API gateway
- RESTful API endpoints (`/server/api/`)
- gRPC server for agent communication (`/server/grpc/`)
- Database layer with Bun ORM (`/server/store/`)
- VCS integration (forge) supporting GitHub, GitLab, Gitea, Forgejo, Bitbucket, Bitbucket Datacenter (`/server/forge/`)
- Pipeline orchestration and scheduling (`/server/pipeline/`)

**Agent** (`/cmd/agent/`, `/agent/`)

- Distributed pipeline execution workers
- Connects to server via gRPC
- Supports multiple execution backends: Docker, Kubernetes, local (`/pipeline/backend/`)
- Handles pipeline step execution with logging

**CLI** (`/cmd/cli/`, `/cli/`)

- Command-line interface for administration and operations
- Repository, pipeline, secret, and user management
- Local pipeline execution and linting

**Web UI** (`/web/`)

- Vue.js frontend with TypeScript
- Real-time pipeline monitoring and management
- Uses Pinia for state management, Vue Router for routing

### Pipeline System

**Frontend Processing** (`/pipeline/frontend/yaml/`)

- YAML pipeline parsing and validation
- DAG (Directed Acyclic Graph) construction for step dependencies
- Matrix workflow support for parallel execution
- Constraint evaluation and linting

**Backend Execution** (`/pipeline/backend/`)

- Pluggable execution backends (Docker, Kubernetes, local, dummy)
- Container and pod lifecycle management
- Volume, network, and secret handling
- Log streaming and status reporting

**Communication** (`/pipeline/rpc/`)

- gRPC Protocol Buffers for agent-server communication
- Structured logging and tracing
- Authentication and authorization

### Data Layer

**Models** (`/server/model/`)

- Core data structures: User, Repo, Pipeline, Step, Task, Agent, etc.
- Database schema definitions with `bun:"..."` struct tags
- JSON-serialized fields use named type aliases (`JSONSliceString`, `JSONMapStringString`, etc.) in `json_types.go`

**Store Interface** (`/server/store/`)

- Unified data access layer with [Bun ORM](https://bun.uptrace.dev/)
- Database migrations (`/server/store/datastore/migration/`)
- Supports SQLite, PostgreSQL, MySQL

### Multi-Forge Support

The system supports multiple forge (VCS) connections per user:

- `user_forges` table stores per-user, per-forge OAuth tokens
- Users can link accounts from multiple forges under one Crow account
- `UserToken()` in `/server/forge/common/utils.go` resolves the correct token for each repo's forge
- Repos have a `forge_id` indicating which forge they belong to

### Key Libraries and Utilities

**Shared** (`/shared/`)

- Common utilities, HTTP helpers, token management
- Logger and terminal utilities

**Version** (`/version/`)

- Version information and build metadata

## Development Patterns

### Testing

- Use table-driven tests for comprehensive coverage
- Mock external dependencies using generated mocks (`mockery`)
- Database tests use test-specific migration scenarios

### Error Handling

- Structured error handling with error wrapping
- Use `github.com/pkg/errors` for error context
- Consistent error responses in API layer

### Logging

- Structured logging with `github.com/rs/zerolog`
- Context-aware logging throughout the system
- Log levels: debug, info, warn, error, fatal

### Database Schema

Key tables include:

- `users` - User accounts and authentication
- `user_forges` - Per-user forge connections and OAuth tokens
- `repos` - Repository configuration and settings
- `pipelines` - Pipeline execution records
- `steps` - Individual pipeline steps
- `tasks` - Task queue for agent execution
- `agents` - Registered agent instances
- `secrets` - Encrypted secret storage
- `registries` - Container registry credentials
- `orgs` - Organizations (synced from forges)

### Database Migrations

Migrations live in `/server/store/datastore/migration/`.
The system uses a pure `database/sql` runner that tracks applied migrations in a `migrations` table.

**Fresh databases** get the full schema from `baseline_schema.sql` (embedded at compile time).
All legacy migration IDs are marked as applied so the runner won't try to re-run them.

**Existing databases** skip the baseline and only run new migrations.

**Adding a new migration:**

1. Open `/server/store/datastore/migration/migration.go`
2. Add a new entry to the `newMigrations` slice in the `Migrate()` function:

   ```go
   {ID: "crow-describe-change", Migrate: func(tx *sql.Tx) error {
       _, err := tx.Exec("ALTER TABLE repos ADD COLUMN new_col TEXT")
       return err
   }},
   ```

3. Add the migration ID to `crowMigrations` in `constants.go`
4. If the migration adds a new table or column, also update `baseline_schema.sql` so fresh databases get the new schema
5. Use the `crow-` prefix for the migration ID to avoid conflicts with Woodpecker migration IDs

Migrations run inside a transaction (`*sql.Tx`) and are recorded atomically.
Use raw SQL only — no ORM calls inside migrations.

## Adding New Features

### Adding New API Endpoints

1. Define model in `/server/model/`
2. Add store methods in `/server/store/`
3. Create API handlers in `/server/api/`
4. Update OpenAPI documentation
5. Add frontend integration if needed

### Frontend Development

1. Vue components in `/web/src/components/`
2. Views in `/web/src/views/`
3. API client in `/web/src/lib/api/`
4. State management with Pinia stores
5. i18n support in `/web/src/assets/locales/`

### Documentation

- Documentation lives in `/docs/` using Astro Starlight
- Use Starlight `<Steps>` component for numbered instructions:

  ```mdx
  import { Steps } from '@astrojs/starlight/components';

  <Steps>1. First step 2. Second step</Steps>
  ```

- New features should be documented in `/docs/src/content/docs/dev/`

**When adding new features, always update documentation:**

- New environment variables → `/docs/src/content/docs/dev/configuration/env-vars/`
- New CLI commands → `/docs/src/content/docs/dev/cli/`
- New pipeline syntax → `/docs/src/content/docs/dev/usage/`
- Server configuration → `/docs/src/content/docs/dev/configuration/server.mdx`
- Backend options (Docker/K8s) → update both env-vars docs and server.mdx backend options section

## Documentation Index

Crow's user-facing documentation lives in `docs/src/content/docs/dev/`. Key files by topic:

### Getting Started

- `quickstart.mdx` — 5-minute setup with Docker Compose
- `index.mdx` — Feature overview, Crow vs Woodpecker differences

### Pipeline Authoring

- `usage/workflow-syntax.mdx` — Complete YAML reference (steps, when filters, matrix, services, depends_on)
- `usage/pipelines.mdx` — Pipeline/workflow/step hierarchy, execution model, manual triggers
- `usage/services.mdx` — Service containers (databases, caches) in pipelines
- `usage/secrets.mdx` — Secret levels (repo/org/global), from_secret usage, plugin filtering
- `usage/env-vars-usage.mdx` — Environment variables in pipelines
- `usage/volumes.mdx` — Volume management
- `usage/advanced.mdx` — Advanced pipeline patterns
- `usage/jsonnet.mdx` — Jsonnet pipeline generation
- `usage/notifications.mdx` — Email notification configuration and per-pipeline overrides
- `usage/appearance.mdx` — UI themes, color mode, per-user theme overrides, and admin-provided themes (`CROW_CUSTOM_THEMES`)
- `usage/metrics.mdx` — Metrics dashboard, chart builder, and historic-data preservation for removed repos/orgs

### Operations and CLI

- `usage/cli.mdx` — CLI installation, configuration, commands
- `usage/repo-settings.mdx` — Repository settings
- `usage/registries.mdx` — Container registry credentials
- `usage/cron.mdx` — Cron job configuration
- `usage/deployment.mdx` — Deployment events and patterns
- `usage/multi-forge.mdx` — Multi-forge setup
- `usage/integrations.mdx` — Third-party integrations

### Server Configuration

- `configuration/server.mdx` — Server configuration reference
- `configuration/agent.mdx` — Agent configuration
- `configuration/autoscaler.mdx` — Autoscaler setup
- `configuration/forge-oauth.mdx` — Forge OAuth setup
- `configuration/oidc.mdx` — Single sign-on through an OpenID Connect provider, groups and admin mapping
- `configuration/env-vars/` — All environment variables by category

### Installation

- `installation/container.mdx` — Docker/Podman installation
- `installation/binary.mdx` — Binary installation
- `installation/helm.mdx` — Kubernetes Helm chart
- `installation/proxy.mdx` — Reverse proxy setup
- `installation/migration.mdx` — Migrating from Woodpecker

### Plugins

- `plugins/index.mdx` — Plugin system overview
- `plugins/using-plugins.mdx` — Using plugins in pipelines
- `plugins/creating-plugins.mdx` — Creating custom plugins
- `plugins/available-plugins.mdx` — Available plugin list
