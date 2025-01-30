## Overview

A pipeline consists of **one or more** workflows.
A workflow consist of **one or more** steps.

!!! important

    Pipeline > Workflow > Step

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

!!! important

    Each workflow is isolated from other workflows with respect to its working directory and file access.
    *Steps* within a workflow have access to the same files.

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

!!! note

    Workflows which depend on others only start if the dependent ones finished successfully.
    A workflow can be forced to execute no matter the exit status of other workflows by setting

    ```yaml
    runs_on: [ success, failure ]
    ```

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
