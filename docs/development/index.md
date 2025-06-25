---
hide:
  - navigation
---

<!-- markdownlint-disable MD041 -->

# Development

## Required tools

The following tools are required for development:

| Technology/Tool                                                 | Component/Purpose  | Comment                                    |
| --------------------------------------------------------------- | ------------------ | ------------------------------------------ |
| [Go](https://go.dev/dl/)                                        | Backend            |                                            |
| [Bun](https://bun.sh/)                                          | Frontend           |                                            |
| [Just](https://github.com/casey/just)                           | Frontend & Backend | modern `make` alternative                  |
| [mkdocs-material](https://github.com/squidfunk/mkdocs-material) | Docs               |                                            |
| [Python](https://www.python.org/downloads/)                     | Docs               | `mkdocs` uses Python                       |
| [fish](https://fishshell.com/)                                  | Docs               | Personal preference, not strictly required |
| [prettier](https://prettier.io/)                                | Lint               |                                            |
| [golangci-lint](https://golangci-lint.run/)                     | Lint               |                                            |
| [pre-commit](https://pre-commit.com/)                           | Lint               | Personal preference, not strictly required |
| [Docker](https://docs.docker.com/get-docker/)                   | Images             | `podman` is not (yet) supported            |

Once `just` is installed, you can install adjacent golang dependencies using the following command:

```sh
just install-dev-deps
```

/// note
`just install-dev-deps` does not install all dev tools listed above but only go-related ones.
You are responsible for installing the complete stack of tools and dependencies on your system.
///

/// tip
If you are having issues installing all dependencies or have others reasons for not installing them, you can also make use of the `.devcontainers.json` and use this one with services that spin up virtual dev environments, e.g. [devpod.sh](https://devpod.sh/)
///

The following just rules are helpful for local development:

```sh
just test-server
just test-agent
just test-cli
just test-server-datastore # test datastore / database related code like migrations

just lint
just lint-backend
just lint-frontend
```

## Building images

/// note
Building images is not required for local development.
You only need to build them if you want to test branch builds on existing Crow instances.
For local development testing, the easiest way is to use the provided `launch.json` config and test the changes in VSCode.
///

Images for dev purposes can be built using

```sh
# server
env PLATFORMS='linux|amd64;linux|arm64' just image-server-alpine <tag>

# agent
env PLATFORMS='linux/amd64,linux/arm64' just image-agent-alpine <tag>

# cli
env PLATFORMS='linux/amd64,linux/arm64' just image-cli-alpine <tag>
```

where `<tag>` is the docker tag to be used for the image.
You do not need to built multiarch for testing, just choose the architecture you want to test the images on.

## VSCode debug/test configuration

`.vscode/launch.json` holds a complete configuration for starting all Crow components locally.
Use the "Crow CI" configuration and then access `localhost:8000` in your browser.

To make this all work, some env vars need to be provided in a local `.env` file.

```sh
export CROW_OPEN=true
export CROW_ADMIN=<your forge username>

export CROW_HOST=http://localhost:8000

# forge config (forgejo) (see /docs/administration/forge/overview for other forges)
export CROW_FORGEJO=true
export CROW_FORGEJO_URL=https://codeberg.org
export CROW_FORGEJO_CLIENT=<id>
export CROW_FORGEJO_SECRET=<secret>

# agent
export CROW_SERVER=localhost:9000
export CROW_AGENT_SECRET=<agent secret>
export CROW_MAX_WORKFLOWS=1

# enable if you want to develop the UI
export CROW_DEV_WWW_PROXY=http://localhost:8010

# if you want to test webhooks with a forge this address needs to be set and accessible from public server
export CROW_EXPERT_WEBHOOK_HOST=

# disable health-checks while debugging (normally not needed while developing)
export CROW_HEALTHCHECK=false

# (optional) when running the vite dev server in a container
# export VITE_DEV_SERVER_HOST=0.0.0.0

export CROW_LOG_LEVEL=debug
```

Notes:

- Workflows will be run on your local machine using the `docker` backend.
- The forge-specific settings (`CROW_FORGEJO_URL`, `CROW_FORGEJO_CLIENT`, `CROW_FORGEJO_SECRET`) must be set if you want to test against existing repositories of a forge. You can use an existing application and append `http://localhost:8000/authorize` to the redirect URI list. See [FIXME] for instructions on how to create an oauth2 app for the respective forge.
- Crow will use a SQLite DB in this scenario, stored at `./crow.sqlite` in the workspace root (and gitignore'd by default). Any backend action you do (e.g. adding secret or enabling a repo) will be persisted in this DB.
- `CROW_EXPERT_WEBHOOK_HOST` sets the URL which incoming webhooks should target, i.e. the local development server. By default this one is running on localhost and accessible from the public. To make this work, you need to expose the local Crow server to the outside world.
  To make this work, you can either

      - run the local development server on `0.0.0.0` and expose the port directly to the     outside world (not recommended without https if you're working with an oauth2 app)

      - run a reverse proxy locally which listens for incoming requests, does SSL termination and forwards the requests to the local development server. This is recommended if you're working with an oauth2 app.

Usually, the easiest way is to not use `CROW_EXPERT_WEBHOOK_HOST` and work with `event: manual` instead.
This event trigger is not dependend on remote webhooks but is triggered from within the Crow server itself.

/// tip
To test if everything works, you can make a simple change to Crow, e.g. adjust a text in the UI (easiest in `en.json`).
When you hit the "refresh" button in the VSCode debug tools toolbar, you should see the UI being refreshed with the change on the fly.
///

## Documentation

When working on the documentation, the following commands can be used:

```sh
# inits and populates the virtual environment
just docs-venv
just docs-serve
```

Crow uses the "insiders" build of `mkdocs-material`, which is only available through a private repository which must be accessed via a token.
The Justfile rules automatically accounts for this and will download the free version of `mkdocs-material` if the token is not available.
Most features for development do not rely on the "insiders" build.

/// info
When you open a PR in the repository, the `surge-preview` plugin will automatically build and deploy the documentation state of the PR to a temporary URL.
This can be an alternative way to work on the documentation if you are facing troubles getting the local development server to work.
///

### Versioning

A new docs version is published to the `gh-pages` branch via [mike](https://github.com/jimporter/mike) for every release and push to `main` (="dev").
(`mike` does not allow to use a different branch name than `gh-pages` as it is heavily focused on GitHub Pages.)
From this branch, the assets will be synced to S3.
