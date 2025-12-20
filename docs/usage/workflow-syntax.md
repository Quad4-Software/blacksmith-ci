<!-- markdownlint-disable MD041 -->

# Workflow syntax

The Workflow section defines a list of steps to build, test and deploy.
Steps are executed serially in the order in which they are defined.
If a step returns a non-zero exit code, the workflow and therefore the entire workflow terminates and returns an error status.

/// note
An exception to this behavior are steps which have `#!yaml status: [failure]` set.
This ensures that they are executed even if a previous step failed.
///

Exemplary step definition:

```yaml
steps:
  - name: backend
    image: <image>
    commands:
      - <command 1>
      - <command 2>
  - name: frontend
    image: <image>
    commands: |
      <command 1>
```

Besides `steps:`, various other top-level commands are understood which allow controlling the pipeline execution flow.

## clone

/// important
A default clone step is already implicitly defined.
This option only needs to be used if the default cloning behavior should be altered.
///

A common use case is to disable partial cloning, to allow for a possible `git push` in a follow-up step (which does not work with a partial clone).

```yaml hl_lines="5"
clone:
  - name: clone
    image: codeberg.org/crow-plugins/clone
    settings:
      partial: false
```

See the [default configuration of the clone plugin](https://woodpecker-ci.org/plugins/Git%20Clone) to check for possible options to change.

## name

The `name` field allows to set a custom name for a pipeline.
By default, the workflow name is derived from the YAML file name (e.g., `build.yaml` → `build`).

For matrix pipelines, a `NAME` (case-insensitive) key can be provided for each matrix entry:

```yaml
matrix:
  include:
    - NAME: name1
      VAR1: value1
    - NAME: name2
      VAR1: value2
```

Both `name` and `NAME` support variable substitution (e.g., `${ENVIRONMENT}`).
If a matrix `NAME` is set, it takes priority over other options.
Otherwise, the top-level `name` is used.
If neither is set, the file name is used.

/// important
`name` and `NAME` are reserved keys and will not be available as environment variables in steps or matrix includes.
If set, they are only used for workflow naming and not passed to the environment.
///

## depends_on

Multiple workflows can be set up for a repository.
Those will run independent of each other by default.
To introduce a dependency among these, the `depends_on` keyword can be used with the workflow name (usually the name of the YAML file).

## labels

Labels can be set for workflows.
These are used by agents to filter their execution.
An agent will pick up and run a workflow when **every label** assigned to it matches the agents labels.

Every workflow has the `repo=your-user/your-repo-name` label set.
If the platform attribute of a workflow has been set, a label like `platform=your-os/your-arch` is set as well.

Labels can be set in a map:

```yaml hl_lines="1 2 3 4"
labels:
  location: europe # only agents with `location=europe` or `location=*` will be used
  weather: sun
  hostname: '' # this label will be ignored as it is empty

steps:
  - name: build
    image: <image>
    commands:
      - go build
      - go test
```

/// important
Setting a custom label also requires assigning this label to at least one agent.
Otherwise, the workflow will never be run.
///

## matrix

Matrix builds allow separate workflows for each combination in the matrix, simplifying building and testing a single commit against multiple configurations.

For more details check the dedicated documentation on [matrix builds](pipelines.md/#matrix-workflows).

## privileged

Steps can be given "privileged" capabilities.
Doing so is a security risk and should only be used in private environments, and even there only as the last resort.

/// info
Adding `privileged: true` in the YAML definition must be combined with enabling security-related project options.
The latter can only be enabled by an instance admin.
This prevents potential abuse by users, as enabling privileged containers combined with running as `root` can potentially lead to a host/agent takeover.
///

/// tip
A secure alternative to allow running privileged steps is to create a plugin and add it to `CROW_PLUGINS_PRIVILEGED`.
A plugin cannot execute arbitrary steps and hence running a privileged container as a plugin highly reduces the attack surface.
///

### Privileged plugin image matching

The environment variable `CROW_PLUGINS_PRIVILEGED` controls which plugin images are allowed to run in privileged mode.
The way images are matched is controlled by `CROW_PLUGINS_PRIVILEGED_MATCH_TYPE` (default: `semver`).

#### Match types

- **semver** (default):
  - `<image>:6` matches all plugin versions with major version 6 (e.g., `<image>:6.1.2`).
  - `<image>:6.3` matches all plugin versions with major version 6 and minor version 3 (e.g., `<image>:6.3.5`).
  - `<image>:6.3.1` matches only the specific version 6.3.1.
- **semver-range**: Uses semver constraint syntax for more flexible version matching:
  - `<image>:>=6` matches any version 6.0.0 or higher.
  - `<image>:<7.0` matches any version less than 7.0.0.
  - `<image>:>=6.0.0,<7.0.0` matches any version from 6.0.0 (inclusive) to 7.0.0 (exclusive).
  - `<image>:~6.3` equivalent to `<image>:>=6.3.0,<6.4.0` (allows patch-level changes).
  - `<image>:^6.3` equivalent to `<image>:>=6.3.0,<7.0.0` (allows minor-level changes).
- **regex**: Each entry in `CROW_PLUGINS_PRIVILEGED` is treated as a regular expression and matched against the full image name (including tag).
- **exact**: Only exact string matches are allowed.

#### Example usage

```sh
export CROW_PLUGINS_PRIVILEGED="codeberg.org/crow-plugins/docker-buildx:1,plugins/gcr:7.2"
export CROW_PLUGINS_PRIVILEGED_MATCH_TYPE=semver
```

- This will allow any `crow-plugins/docker-buildx` image with a major version 1 tag, and any `plugins/gcr` image with a 7.2.x tag, to run in privileged mode.

```sh
export CROW_PLUGINS_PRIVILEGED="codeberg.org/crow-plugins/docker-buildx:1\\..*"
export CROW_PLUGINS_PRIVILEGED_MATCH_TYPE=regex
```

- This will allow any `crow-plugins/docker-buildx` image with a tag starting with 1.

```sh
export CROW_PLUGINS_PRIVILEGED="codeberg.org/crow-plugins/docker-buildx:1.0.0"
export CROW_PLUGINS_PRIVILEGED_MATCH_TYPE=exact
```

- This will only allow the exact image `codeberg.org/crow-plugins/docker-buildx:1.0.0` to run in privileged mode.

```sh
export CROW_PLUGINS_PRIVILEGED="codeberg.org/crow-plugins/docker-buildx:>=1.0.0,<2.0.0"
export CROW_PLUGINS_PRIVILEGED_MATCH_TYPE=semver-range
```

- This will allow any `crow-plugins/docker-buildx` image with a version from 1.0.0 (inclusive) to 2.0.0 (exclusive) to run in privileged mode.

## runs_on

Workflows that should run even if dependent workflows failed.

```yaml
runs_on: [success, failure]
```

See the ["Execution control"](pipelines.md/#execution-control) page for more details.

## services

Service containers can be used to asynchronously run databases or cache containers during the execution of a workflow.

For more details check the [services docs](services.md).

## skip_clone

The implicit clone step, which can be configured through the [`clone`](#clone) keyword, can be skipped entirely.

```yaml
skip_clone: true
```

/// tip
When running with the Kubernetes backend, skipping the automatic cloning can save various seconds (especially when using a RWX for the temporary workflow volume).

The clone step is often done on a different node than the actual workflow steps, and the mount/umount operation across nodes might take some time.
It can therefore be beneficial to skip it and add another manual `git clone <repo>` command as the first command in the workflow.
///

## variables

Variables are global workflow definitions which can be reused across the workflow definition.

Advanced YAML features such as _anchors_ and _aliases_ are supported.

For more details and examples see the [advanced usage](advanced.md) documentation.

## when

The `when:` defines the events when a workflow is triggered.
If at least one of the conditions in the `when` block evaluates to true, the step is executed, otherwise it is skipped.
A condition is evaluated to `true` if all sub-conditions are true.

/// important
Skipped workflows do not block other dependent workflows from starting.
///

```yaml
when:
  - event: pull_request
  - event: push
    branch: ${CI_REPO_DEFAULT_BRANCH}
```

The above example would trigger the workflow

- on `pull_request` events
- on `push` events to the default branch of the repository

For more information about the specific filter combinations, take a look at the [step-specific `when` filters](#when-step).

## workspace

The workspace defines the shared volume and working directory which is shared by all workflow steps.
The default workspace base is `/crow` and the path is extended with the repository URL (`src/{url-without-schema}`).
A full example would be `/crow/src/github.com/octocat/hello-world`.

/// note
This setting usually does not need to be altered.
Plugins will always have the workspace base at `/crow`.
///

The base attribute defines a shared base volume available to all steps.
This ensures that source code, dependencies and other dynamic assets are persisted and shared between steps.

```yaml hl_lines="2"
workspace:
  base: /go
  path: src/github.com/octocat/hello-world
```

The `path` attribute defines the working directory of the build.
The value of `path` must be relative and is combined with the `base` path.

## steps

### commands

The commands of every step are executed serially as if they would be entered into a terminal.

```yaml
steps:
  - name: backend
    image: <image>
    commands:
      - go build
      - go test
```

Behind the scenes, the commands are converted into a shell script, which is then executed as the container entrypoint (`docker run --entrypoint=build.sh golang`):

```sh
#!/bin/sh
set -e

go build
go test
```

Sometimes (e.g. to avoid issues with escaping quotes and double-colons) it can be easier to specify all commands as a block instead of a list in YAML:

```yaml
steps:
  - name: backend
    image: <image>
    commands: |
      go build
      go test
```

### depends_on { #depends_on_steps }

Normally, workflow steps are executed serially in the order in which they are defined.
As soon as `depends_on` is set for one or more steps, a [directed acyclic graph](https://en.wikipedia.org/wiki/Directed_acyclic_graph) (DAG) will be created and all steps of the workflow will be executed in parallel.
Only the steps that have a dependency set to another step via `depends_on` are executed serially:

```yaml hl_lines="11"
steps:
  - name: build # will be executed immediately
    image: <image>
    commands:
      - go build

  - name: deploy
    image: <image>
    settings:
      repo: foo/bar
    depends_on: [build, test] # deploy will be executed after 'build' and 'test' finished

  - name: test # will be executed immediately as no dependencies are set
    image: <image>
    commands:
      - go test
```

/// tip
A step can be started immediately by using an empty `depends_on: []`.
Doing so for a single step will cause all other steps to be executed immediately if no further dependencies are specified.
///

### detach

The `detach` options can be used to run steps in the background until a workflow has finished.
It is similar to `services:` and hence explained in more detail in the ["services" documentation](services.md#detachment).

### directory

The `directory` options allows setting a subdirectory inside the container in which the commands will be executed.

### entrypoint

The container entrypoint.

/// important
This must be a list of the command and its arguments, e.g., `["/bin/sh", "-c"]`.
///

The default entrypoint is

```sh
"/bin/sh", "-c", "echo $CI_SCRIPT | base64 -d | /bin/sh -e"
```

An alternative to the above which allows defining custom parameters is to use a custom shell via the env var `CI_SCRIPT` (Base64-encoded).

### environment

Pass arbitrary environment variables to individual steps.

```yaml
steps:
  - name: backend
    image: <image>
    environment:
      XY: true
    commands: |
      go build
      go test
```

For more details, check the dedicated ["environment variables"](env-vars-usage.md) documentation.

### failure

Allow steps to fail without causing the whole workflow and the subsequent pipeline to fail (e.g., a step executing a linting check).

If an error is encountered while executing the step, the step will be reported as "failed" but the next steps of the workflow will continue to execute.

```yaml hl_lines="7"
steps:
  - name: backend
    image: <image>
    commands:
      - go build
      - go test
    failure: ignore
```

### image

The container image to use for the step.

/// important
When using the local backend, the `image` value is used to specify the shell, such as `bash` or `fish`.
///

```yaml
steps:
  - name: build
    image: <image>

services:
  - name: database
    image: mysql
```

### volumes

Additional volumes can be mounted to individual steps.

For the `docker` backend, this option can be used to mount files or folders from the host machine into the respective containers.

For the `kubernetes` backend, an existing persistent volume claim (PVC) is used.

More details can be found in the [volumes documentation](volumes.md).

### when {#when-step}

Woodpecker supports defining a list of conditions for a step by using a when block.
If **at least one** of the conditions in the `when` block evaluate to true, the step is executed, otherwise it is skipped.
A condition is evaluated to `true` if all sub-conditions are true.

#### branch

/// important
The step below is triggered on branch `main`.
Besides normal "push" events, this also includes pull requests for which the target branch is `main`.
///

```yaml
steps:
  - name: backend
    image: <image>
    commands:
      - go build
      - go test
    when:
      - branch: main
```

To only run for pushes to `main`, the following would need to be set:

```yaml
steps:
  - name: backend
    image: <image>
    commands:
      - go build
      - go test
    when:
      - branch: main
        event: push
```

A condition can also be a list:

```yaml
steps:
  - name: backend
    image: <image>
    commands:
      - go build
      - go test
    when:
      - branch: [main, dev]
        event: push
```

Regex is also supported:

```yaml
steps:
  - name: backend
    image: <image>
    commands:
      - go build
      - go test
    when:
      - branch: prefix/*
        event: push
```

The matching is done via [bmatcuk/doublestar/](https://github.com/bmatcuk/doublestar/#usage).
Please check its documentation for complex condition sets.

An example which includes both explicit `exclude` and `include` parts could be written as:

```yaml
when:
  - branch:
      include: [main, release/*]
      exclude: [release/1.0.0, release/1.1.*]
```

#### cron

/// important
Make sure to also define a global `cron` event trigger.
///

```yaml
when:
  - event: cron
    cron: <cron name>
```

#### evaluate

Execute a step only if the provided evaluate expression equals to `true`.
Both built-in `CI_` and custom variables can be used inside the expression.

Examples:

- Run on pushes to the default branch for the repository `owner/repo`:

      ```yaml
      when:
        - evaluate: 'CI_PIPELINE_EVENT == "push" && CI_REPO == "owner/repo" && CI_COMMIT_BRANCH == CI_REPO_DEFAULT_BRANCH'
      ```

- Run on commits created by user `user1`:

      ```yaml
      when:
        - evaluate: 'CI_COMMIT_AUTHOR == "user1"'
      ```

- Skip all commits containing "please ignore me" in the commit message:

      ```yaml
      when:
        - evaluate: 'not (CI_COMMIT_MESSAGE contains "please ignore me")'
      ```

- Run on pull requests with the label `deploy`:

      ```yaml
      when:
        - evaluate: 'CI_COMMIT_PULL_REQUEST_LABELS contains "deploy"'
      ```

- Skip step only if `SKIP=true`, run otherwise or if undefined:

      ```yaml
      when:
        - evaluate: 'SKIP != "true"'
      ```

#### event

A list of all available events:

- `push`: triggered when a commit is pushed to a branch.
- `pull_request`: triggered when a pull request is opened or a new commit is pushed to it.
- `pull_request_closed`: triggered when a pull request is closed without being merged.
- `pull_request_merged`: triggered when a pull request is merged.
- `pull_request_edited`: triggered when a pull request is edited (the PR title or body).
- `tag`: triggered when a tag is pushed.
- `release`: triggered when a release, pre-release or draft is created. (Further filters can be applied using [evaluate](#evaluate) in combination with environment variables.)
- `deployment`: triggered when a "deployment" is created in the repository. (There are two ways to trigger this event: from the Crow UI directly or from a forge which supports "deployment" events (e.g., GitHub))
- `cron`: triggered when a cron job is executed. Requires a cron job to be configured in the dashboard.
- `manual`: triggered when a user manually triggers a pipeline in UI or via CLI.

/// info
If you want to run pipelines via the "Manual pipeline run" button in the UI, make sure to add the `manual` event trigger to the `when:` section.
///

Event triggers can be combined.
If this is used, **all** conditions need to evaluate to `true`.
Example: Execute a step if the pipeline event is a push to a specified branch:

```yaml
when:
  - event: push
    branch: main
```

Execute a step for multiple events:

```yaml
when:
  - event: [push, tag, deployment, manual]
  # alternative syntax
  # - event:
  #     - push
  #     - tag
  #     - deployment
  #     - manual
```

#### matrix { #matrix_step }

Execute a step for a single matrix permutation:

```yaml
when:
  - matrix:
      GO_VERSION: 1.5
      REDIS_VERSION: 2.8
```

#### path

Only execute a step when specific files were changed:

```yaml
when:
  - path: 'src/*'
```

[Glob patterns](https://github.com/bmatcuk/doublestar#patterns) are used to match the changed files.

For pipelines without file changes (empty commits or on events without file changes like `tag`), `on_empty` can be used to set whether the condition should be `true` (default) or `false`.

```yaml
when:
  - path:
      include: ['.crow/*.yaml', '*.ini']
      exclude: ['*.md', 'docs/**']
      ignore_message: '[ALL]'
      on_empty: true
```

/// important
Passing a custom ignore-message like `[ALL]` inside the commit message will take precedence and ignore all `path` conditions and the `on_empty` setting.
///

#### platform

/// important
This filter should not be used with the kubernetes backend.
Instead, use affinity and nodeSelector options within the `backed_options`.
///

Execute a step for a specific platform:

```yaml
when:
  - platform: linux/amd64
```

Execute a step for a specific platform using wildcards:

```yaml
when:
  - platform: ['linux/*', 'windows/amd64']
```

#### ref

The `ref` filter is compared against the git reference of the workflow.
Example: filter tags that start with `v`:

```yaml
when:
  - event: tag
    ref: refs/tags/v*
```

#### repo

Filter by repository:

```yaml
when:
  - repo: test/test
```

#### status

To execute steps on failure of previous pipelines, for example to send notifications for such, the `status` filter can be used:

```yaml
when:
  - status: [success, failure]
```
