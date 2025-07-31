## Overview

A pipeline consists of **one or more** workflows.
A workflow consist of **one or more** steps.

/// important
Pipeline > Workflow > Step
///

A YAML file in `.crow/` defines _one or more_ workflow(s) ("more" if a "matrix" workflow is defined).
The following file tree would consist of _four_ workflows:

```tree
.crow/
├── build.yaml
├── deploy.yaml
├── lint.yaml
└── test.yaml
```

Each workflow can consist of an arbitrary number of steps.
By default, workflows do not have a dependency to each other and are executed in parallel.
Steps within a workflow are executed sequentially by their order of definition.

Both steps and workflows accept a `depends_on: []` key which can be used to specify an explicit execution order.

/// important
Each workflow is isolated from other workflows with respect to its working directory and file access.
_Steps within a workflow_ have access to the same files as they share a common workflow-specific volume which is attached to all steps within the workflow.
///

## Execution control

By default, all workflows start in parallel if they have matching event triggers.
An execution order can be enforced by using `depends_on`:

```yaml hl_lines="7 8 9 10"
steps:
  - name: deploy
    image: <image>:<tag>
    commands:
      - some command

# these are names of other workflows
depends_on:
  - lint
  - build
  - test
```

This keyword also works for dependency resolution with steps.

/// note
Workflows which depend on others only start if the dependent ones finished successfully.
A workflow can be forced to execute no matter the exit status of other workflows by setting

```yaml
runs_on: [success, failure]
```

///

## Event triggers

Event triggers are _mandatory_ and define under which conditions a workflow is executed.
At the very least one even trigger must be specified, for example to execute the pipeline on a `push` event:

```yaml
when:
  event: push
```

Typically, you want to use a more fine-grained logic including more events, for example triggering a workflow for `pull_request` events and pushes to the default branch of the repository:

```yaml
when:
  - event: pull_request
  - event: push
    branch: ${CI_REPO_DEFAULT_BRANCH}
```

There are more ways to define event triggers using both list and map notation.
Please see FIXME for all available options.

## Matrix workflows

Matrix workflows execute a separate workflow for each combination in the specified matrix.
This simplifies testing and building against multiple configurations without copying the full pipeline definition but only declare the variable parts.

Example:

```yaml
matrix:
  GO_VERSION:
    - 1.4
    - 1.3
  REDIS_VERSION:
    - 2.6
    - 2.8
    - 3.0
```

Each definition can also be a combination of variables.
In this case, nest the definitions below the `include` keyword:

```yaml hl_lines="2"
matrix:
  include:
    - GO_VERSION: 1.4
      REDIS_VERSION: 2.8
    - GO_VERSION: 1.5
      REDIS_VERSION: 2.8
```

### Interpolation

Matrix variables are interpolated in the YAML using the `${VARIABLE}` syntax, **before the YAML is parsed**.
This is an example YAML file **before** interpolating matrix parameters:

```yaml
matrix:
  GO_VERSION:
    - 1.4
    - 1.3
  DATABASE:
    - mysql:8
    - mysql:5
    - mariadb:10.1

steps:
  - name: build
    image: golang:${GO_VERSION}
    commands:
      - go get
      - go build
      - go test

services:
  - name: database
    image: ${DATABASE}
```

And after:

```yaml hl_lines="3 9 10 14"
steps:
  - name: build
    image: golang:1.4
    commands:
      - go get
      - go build
      - go test
    environment:
      - GO_VERSION=1.4
      - DATABASE=mysql:8

services:
  - name: database
    image: mysql:8
```

### Examples

- Matrix pipeline with a variable image tag:

  ```yaml
  matrix:
    IMAGE:
      - golang:1.7
      - golang:1.8
      - golang:latest

  steps:
    - name: build
      image: ${IMAGE}
      commands:
        - go build
        - go test
  ```

- Matrix pipeline using multiple platforms:

  ```yaml
  matrix:
    platform:
      - linux/amd64
      - linux/arm64

  steps:
    - name: test
      image: <image>
      commands:
        - echo "Running on ${platform}"

    - name: test-arm
      image: <image>
      commands:
        - echo "Running on ${platform}"
      when:
        platform: linux/arm*
  ```

  /// tip
  For the kubernetes backend, architecture-specific pipelines should be controlled via the [`nodeSelector` backend option](../configuration/server.md#backend-options-backend-options-kubernetes).
  ///

## Skipping commits

Commits can be prohibited from triggering a webhook by adding `[SKIP CI]` or `[CI SKIP]` (case-insensitive) to the commit message.

## Container Naming Scheme

Crow supports configurable container naming schemes which determines the names of containers (and pods/services in Kubernetes) created during pipeline execution.

There are two supported schemes:

1. Descriptive (default)

   - **Format:** `<owner>-<repo name>-<pipeline id>-<workflow id>-<step name>`
   - **Example:** `myowner-myrepo-42-3-build`
   - **Matrix workflows:** Each workflow instance gets a unique workflow number for proper identification
   - **Single workflows:** Workflow number is still included for consistency

2. Hash-based (legacy)

   - **Format:** `crow_<hash>`
   - **Example:** `crow_123e4567-e89b-12d3-a456-426614174000`
   - This was previously the default (until 3.x) (as `wp_<hash>`), now updated to `crow_` for clarity.

/// note
Names are sanitized and truncated to 64 characters for Kubernetes compatibility.
///

The naming scheme can be set via the server environment variable `CROW_CONTAINER_NAME_SCHEME`.
