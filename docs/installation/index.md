---
title: Installation
---

## General

Crow consists out of essential components (the "server" and the "agent") and an optional one (the "autoscaler").

The **server** provides the UI, handles webhook requests to the underlying forge, serves the API and parses the pipeline configurations from the YAML files.

The **agent** executes the pipelines using a specific backend (`docker`, `kubernetes`, `local`) and connects to the server via GRPC.
Multiple agents can coexist besides each other, allowing to fine-tune job limits, backend choice and other agent-related settings for a single instance.

The **autoscaler** allows spinning up new VMs on a cloud provider of choice to process pending builds.
After the builds finished, the VMs are destroyed again (after a short transition time).

Crow ships and uses a SQLite DB by default.
For larger instances it is recommended to use it with a Postgres or MariaDB instance.[^2]

/// note
The deployment of an external database is not covered here. There are many existing public guides for deploying databases. An alternative option is to use a managed DB service from a Cloud provider. If you are unsure what you need and if Crow is a good fit for you in general, you can also proceed with the SQLite DB first and decide later.
///

There are currently two official ways[^1] how to install Crow:

- Via [`docker-compose`](docker-compose.md) for single servers.
- Via [`helm`](helm.md) for Kubernetes.

## Crow agent tokens

To allow secure communication between the _server_ and _agent_ via GRPC, a token is required.

There are two types of tokens:

- System token
- Agent token

When using the Helm chart, a Kubernetes secret containing an agent token is created automatically.
This token is then used by all agents which have access to this secret.
In the best case, no further configuration is required.

### System token

The system token is set via the env var `CROW_AGENT_SECRET` for both server and agent deployments.
**There can only ever be one system token at the same time, therefore.**

If a system token is set, the registration process is as follows:

1. The first time the agent communicates with the server, it is using the system token
2. The server registers the agent in its database and generates a unique ID which is then sent back to the agent
3. The agent stores the received ID in a config file (path is defined by `CROW_AGENT_CONFIG_FILE`)
4. At the subsequent starts of the agent, it uses this token and its received ID to identify itself to the server

/// info
If the ID is not stored/persisted in `CROW_AGENT_CONFIG_FILE` and the agent connects with a matching agent token, a new agent is registered in the DB and the UI.
This will happen every time the agent container is restarted.
While this is not an issue at runtime, the list of registered agents will grow and leave behind "zombie" agents which are registered in the DB but not active anymore.
It is therefore recommended to persist `CROW_AGENT_CONFIG_FILE` to ensure idempotent agent registrations.
///

### Agent token

Agent tokens come into play when one wants to provision multiple agents, which, optionally, have certain restrictions set with respect to which workflows they are processing.

Such tokens can be created in the UI of the server (`Settings -> Agents -> Add agent`) or through the API.

The resulting tokens can be handed over to individual agents and referenced via `CROW_AGENT_SECRET`.
Once an agent connects to the server for the first time with a matching agent token, the server registers the agent and allows it to process workflows.

![Agent creation](img/new-agent-registration.png){ width="500" }
/// caption
Registration of a new agent through UI
///

[^2]: This is primarily because Crow (still) stores pipeline logs in the DB. A refactoring to storing these outside the DB by default is planned but not yet implemented.

## Installing agents on separate machines

/// note
Ensure you register an agent in the Crow server first and then use the resulting agent token when connecting the remote agent.
///

The simplest and most straightforward setup is to deploy agents on the same machine where the Crow server is running.
In this case, agents can connect directly to the server's GRPC port (default `9000`) over a local connection.

Yet, there are several reasons to deploy agents on remote servers and have them process builds.
First and foremost, you might want to do this if the Crow server is deployed on a small machine (or with other low-resource deployments) but builds need to be run on more powerful hardware.

In such scenarios, the connectivity must happen between two different machines, which are possibly connected over the public internet.
In the following sections, we'll explore both scenarios: connecting an agent running on a separate machine over an internal network, and connecting via a public internet connection.

### Internal

In this case, the setup is not much different from the default scenario of running the agent alongside the server on the same machine.
The agent must be deployed with the following env vars:

```yaml
# address of the Crow server: private ip (e.g. 10.x.x.x) + port
CROW_SERVER: <private server ip>:8000
CROW_GRPC_ADDR: <private server ip>:9000
# agent token, previously created in Crow server
CROW_AGENT_SECRET: <token>
# (optional) agent labels to restrict workflows processing to matching labels
CROW_AGENT_LABELS: <labels>
# [...] additional agent settings
```

### Public

When the public route must be used (which is the case when adding an agent without controlling the Crow server instance), the process becomes a bit more complex.
This is because a secure SSL connection must be used for the GRPC connection between the server and agent, as otherwise the token exchanged between both could be intercepted.

When using an SSL-GRPC connection, you must have a TLS-ready ingress on the Crow server machine that can process incoming requests adequately.
This means, there must be

- a subdomain for the GRPC service of Crow server
- an SSL certificate for this subdomain

Once this exists, the agent configuration would look as follows:

```yaml
CROW_SERVER: crow.mydomain.com
CROW_GRPC_ADDR: grpc.crow.mydomain.com
CROW_GRPC_SECURE: 'true'
# agent token, previously created in Crow server
CROW_AGENT_SECRET: <token>
# (optional) agent labels to restrict workflows processing to matching labels
CROW_AGENT_LABELS: <labels>
# [...] additional agent settings
```

//// warning
Do not prepend "https" before the domain name for the `CROW_SERVER` var.
////

Some notes:

- `CROW_GRPC_SECURE` is important here as it tells the agent to use an SSL-backed connection when trying to establish the connection.
- If you struggle to configure a proxy server on port 443 taking the subdomain requests, you can also use something like `crow.mydomain.com:<port>`.
  The important part is that the receiving proxy is able to handle the TLS termination and forward the request to the Crow server service.

/// tip
Examples of common GRPC-based ingress configurations are provided in the [Reverse proxy setup](./proxy.md) page.
///

## Image tags

/// info
No `latest` tag exists to prevent accidental major version upgrades.
Either use a Semver tag or one of the rolling major/minor version tags.
Alternatively, the `dev` tag can be used for rolling builds from the `main` branch.
///

- `vX.Y.Z`: SemVer tags for specific releases, no entrypoint shell (scratch image)
  - `vX.Y`
  - `vX`
- `vX.Y.Z-alpine`: SemVer tags for specific releases, rootless for Server and CLI
  - `vX.Y-alpine`
  - `vX-alpine`
- `dev`: Built from the `main` branch
  - `dev-<hash>`
- `pull_<PR_ID>`: Images built from Pull Request branches.

## Image registries

Images are currently solely provided through the Codeberg Container Registry (codeberg.org).

/// info
If there funds available to cover a DockerHub team subscription, images will also be there.
///

- [crow-server (codeberg.org)](https://codeberg.org/crowci/crow/pkgs/container/crow-server)

- [crow-agent (codeberg.org)](https://codeberg.org/crowci/crow/pkgs/container/crow-agent)

- [crow-cli (codeberg.org)](https://codeberg.org/crowci/crow/pkgs/container/crow-cli)

- [crow-autoscaler (codeberg.org)](https://codeberg.org/crowci/crow/pkgs/container/crow-autoscaler)

[^1]: An Ansible role is in the works.
