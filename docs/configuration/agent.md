<!-- markdownlint-disable MD041 -->

To configure an agent, the following settings are required at a minimum:

```sh
CROW_SERVER=<connection uri> # usually 'e.g. woodpecker-server:9000'
CROW_AGENT_SECRET="<token>"
```

## Workflows per agent

/// note
A pipeline often consists of multiple workflows.
It is recommended therefore to increase this setting to a value matching the resources of the surrounding environment (i.e. if builds are executed on a server with 4GB memory, allowing 100 workflows in parallel is likely not a good match).
///

By default, every agent is allowed to execute on workflow at a time.
This can be controlled via `CROW_MAX_WORKFLOWS` to increase the amount of parallel workflows an agent is allowed to process.

## Workflow filters

Crow CI provides two ways to influence which agent processes a specific workflow:

1. Via agent/workflow labels
1. Via agent priorities

/// important
Filtering via labels takes priority.
If an agent is filtered out at the label stage, priority filtering will not have any effect.
///

### Label filtering

Agents can be configured to only process workflows with matching labels.
Each agent has the following default labels:

- backend
- type
- platform
- agent

besides these, admins can add custom labels to agents during registration via `CROW_AGENT_LABELS`:

```sh
CROW_AGENT_LABELS='my-label=my-value'
```

This config would restrict an agent to builds of the label `my-label`.

For these custom labels, the agents must have the same label set in their configuration.
For example, only workflows which have the following label definition set in the YAML config will be processed by the agent with the label defined in this example:

```yaml
labels:
  my-label: my-value
```

A common use case for a custom label is to restrict an agent to to workflows with either low or high resource requirements.

### Priority filtering

If multiple agents are eligible for a workflow, one can use the `priority` label to prioritize a specific agent.
This can be used to steer workflows to agents with more capacity *if* they are available for a given task.

/// note
Besides label filtering, an agent might also be temporarily unavailable to process a workflow because it has reached its maximum workflow capacity.
///

## Organization & user agents

Non-admin users can add agents which are scoped to workflows of specific organizations and/or their own user.

/// important
Agents added via this approach can only ever process workflows of the org/user they are added to.
This is for security reasons; to avoid being able to process workflows of other organizations or users and keep the scope of the agent limited and clear.
///

/// tip
If multiple agents are present, the agent selection is done randomly.
In an instance where global agents are present, it is recommended to add a custom label to org/user agents to ensure that workflows are only processed by the intended agent.
///

When adding remote organization or user agents, i.e., agents which do not run on the same machine as the Crow server, a secure GRPC connection must be established.
The essential configurations that must be present from external agents are:

```yaml
CROW_GRPC_ADDR=
CROW_GRPC_SECURE=true
CROW_AGENT_SECRET=
CROW_BACKEND=
```

Important: do not add a protocol prefix (e.g. https) to the `CROW_GRPC_ADDR` value.

## Multi-org agents

/// info
Admin-only feature
///

Multi-org agents can process workflows of multiple organizations/users/repositories.
Due to their flexibility, they can only be added from instance admins.
This architecture offers a balanced approach - granting targeted access to specific organizations or repositories while maintaining centralized agent administration at the administrative level.

Compared to org/user agents, multi-org agents reduce the number of separate agents needed when handling builds from different organizations within the same internal (meta) team.

/// note
Besides the `org` label, the `repo` label can be used to limit the scope of the agent to a specific repository.
///

Multi-org agents operate using partial matching for the `org` and `repo` labels.
The respective labels can be provided through the `CROW_CUSTOM_LABELS` env var in two ways:

```yaml
CROW_CUSTOM_LABELS='org=1|2|3'
```

or

```yaml
CROW_CUSTOM_LABELS='org=1,org=2,org=3'
```

(The latter syntax will be merged into a final label which looks like the first example.)

The custom labels will be shown in the agent list and highlighted in a different color.

![Multi-org Agent](img/multi-org-agent.png)
/// caption
Multi-org Agent
///

## Backend options

Global backend options, like setting limits for cpu and memory per container, can be defined on the agent level.
All available options are shown in the ["All environment variables"](env-vars.md) section with the backend-specific ones prefixed with `CROW_BACKEND_`.

### Local backend sandboxing (macOS only)

When using the `local` backend on macOS, you can enable process isolation using macOS's built-in `sandbox-exec` utility.
This provides an additional security layer for running workflows locally.

/// info
Sandboxing is only available on macOS and automatically disabled on other platforms.
///

Configure the sandbox level using:

```sh
CROW_BACKEND_LOCAL_SANDBOX_LEVEL=standard
```

#### Available sandbox levels

- `none`

  No sandboxing is applied. Workflows run with full access to the system.

  /// warning
  This runs processes directly without any isolation.
  Only use this in private repos where all actors are trusted and expect potentially     destructive operations to the host system.
  ///

- `standard`

  Balanced security suitable for most CI/CD workloads.

  Allows:

  - ✅ Network access (npm install, go get, pip install, etc.)
  - ✅ Reading system libraries and frameworks
  - ✅ Reading shell configuration files (.bashrc, .zshrc, etc.)
  - ✅ Full access to workflow temporary directories
  - ✅ Executing system binaries and workflow tools

  Denies:

  - ❌ Writing outside workflow directories
  - ❌ Reading sensitive user directories (Documents, Desktop, Downloads)
  - ❌ Reading SSH private keys

- `strict`

  Maximum isolation with minimal permissions.

  Allows:

  - ✅ Reading only essential system files
  - ✅ Full access only to workflow directories
  - ✅ Minimal process execution

  Denies:

  - ❌ All network access
  - ❌ All access to user directories
  - ❌ Reading most system configuration

/// note
`strict` mode blocks network access, which will conflict with most CI/CD workflows that need to download dependencies.
Only use this level if you specifically need maximum isolation and your workflows can run offline.
///

#### Platform detection

The local backend automatically detects when it's running on macOS using Go's build tags.
The sandbox implementation is compiled for the agent binaries only on Darwin (macOS) platforms.
