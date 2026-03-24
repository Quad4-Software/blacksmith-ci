---
name: crow-usage
description: Use when writing Crow CI pipeline YAML, using the Crow CLI, configuring Crow server/agent, or answering questions about Crow CI features and usage
---

# Crow CI Usage

Crow is a self-hosted CI/CD system. Pipelines are defined in `.crow/` YAML files in a repository.

## Pipeline YAML Quick Reference

Pipeline files live in `.crow/` (e.g. `.crow/build.yaml`). Each file defines one workflow containing sequential steps:

```yaml
when:
  - event: [push, pull_request, tag, manual, cron]
    branch: main

steps:
  - name: build
    image: golang:1.23
    commands:
      - go build ./...

  - name: test
    image: golang:1.23
    commands:
      - go test ./...
    when:
      - event: push
        branch: main

  - name: deploy
    image: plugins/docker
    settings:
      repo: myorg/myapp
      tags: latest
    secrets: [docker_username, docker_password]
    when:
      - event: tag
```

**Key elements:**

- `when` — conditional execution (event, branch, path, status, cron, evaluate, matrix, platform, ref, repo)
- `image` — Docker image for the step
- `commands` — shell commands to run
- `secrets` — inject secrets via `from_secret:` in environment or `secrets:` list
- `settings` — plugin configuration
- `depends_on` — step-level DAG dependencies within a workflow
- `services` — sidecar containers (databases, caches) accessible by DNS hostname
- `matrix` — run workflow across multiple variable combinations
- `notify` — per-pipeline email notification overrides (disable or set events: failure, success, status_change)

**Workflow-level features:**

- Multiple `.crow/*.yaml` files run as parallel workflows
- Use top-level `depends_on` to order workflows
- `labels` to target specific agents

For full syntax: read `docs/src/content/docs/dev/usage/workflow-syntax.mdx`

## CLI Quick Reference

Configure with `CROW_SERVER` and `CROW_TOKEN` environment variables, or use `--server` and `--token` flags.

| Task             | Command                                                       |
| ---------------- | ------------------------------------------------------------- |
| List repos       | `crow-cli repo ls`                                            |
| Repo info        | `crow-cli repo info <owner/name>`                             |
| Sync repos       | `crow-cli repo sync`                                          |
| List pipelines   | `crow-cli pipeline ls <owner/name>`                           |
| Trigger pipeline | `crow-cli pipeline start <owner/name>`                        |
| Stop pipeline    | `crow-cli pipeline stop <owner/name> <number>`                |
| Add secret       | `crow-cli secret add <owner/name> --name <key> --value <val>` |
| Remove secret    | `crow-cli secret rm <owner/name> --name <key>`                |
| Lint pipeline    | `crow-cli lint <file>`                                        |
| Run locally      | `crow-cli exec <file>`                                        |
| Agent list       | `crow-cli admin agent ls`                                     |
| User list        | `crow-cli admin user ls`                                      |

For full CLI docs: read `docs/src/content/docs/dev/usage/cli.mdx`

## Common Patterns

**Using secrets in environment:**

```yaml
steps:
  - name: deploy
    image: alpine
    environment:
      API_KEY:
        from_secret: api_key
```

**Service containers:**

```yaml
services:
  - name: database
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: test

steps:
  - name: test
    image: golang:1.23
    commands:
      - go test ./... # connects to "database:5432"
```

**Matrix builds:**

```yaml
matrix:
  GO_VERSION: ['1.22', '1.23']
  DB: [sqlite, postgres]

steps:
  - name: test
    image: golang:${GO_VERSION}
    commands:
      - go test ./...
```

## Documentation

For detailed reference on any topic, read the relevant file from `docs/src/content/docs/dev/`.
See the Documentation Index in CLAUDE.md for a full topic-to-file mapping.
