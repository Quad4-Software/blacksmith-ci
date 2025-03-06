## Overview

A pipeline consists of **one or more** workflows.
A workflow consist of **one or more** steps.

/// important
Pipeline > Workflow > Step
///

A YAML file in `.crow/` defines _one workflow_.
The following file tree would consist of _four_ workflows:

```tree
.crow/
├── build.yaml
├── deploy.yaml
├── lint.yaml
└── test.yaml
```

Each workflow can be

- run individually
- run in parallel
- run in sequence of others

depending on the configuration of the respective _event triggers_ and `depends_on` configuration in each individual workflow.

/// important
Each workflow is isolated from other workflows with respect to its working directory and file access.
*Steps* within a workflow have access to the same files.
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
runs_on: [ success, failure ]
```
///

Changes to files are persisted throughout the steps of a workflow as the same (temporary) volume is mounted to all steps.

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
    For the kubernetes backend, architecture-specific pipelines should be controlled via the [`nodeSelector` backend option](../configuration/server.md#backend-options-backend-options-kubernetes.
    ///

## Skipping commits

Commits can be prohibited from triggering a webhook by adding `[SKIP CI]` or `[CI SKIP]` (case-insensitive) to the commit message.
