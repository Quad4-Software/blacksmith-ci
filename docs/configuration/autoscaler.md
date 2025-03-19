## Preface: What is the 'Crow CI Autoscaler'?

The [Crow CI Autoscaler](https://github.com/crowci/autoscaler) is a standalone application which will spin up servers in a configured cloud provider to execute Crow CI pipelines.
After the VM is ready, it will create a Crow CI agent that will pick up pending pipeline jobs.

If no new builds are in the queue, the Autoscaler will wait a bit (default 10 mins) and then remove the server again.

The app allows you to utilize powerful servers on-demand and terminate them immediately after use.
This minimizes costs and prevents the expense of maintaining large idle instances when no builds are running.

The Crow CI Autoscaler currently works with the following cloud providers:

- [AWS](https://github.com/crowci/autoscaler/blob/main/providers/aws/flags.go)
- [Hetzner Cloud](https://github.com/crowci/autoscaler/blob/main/providers/hetzner/flags.go)
- [Linode](https://github.com/crowci/autoscaler/blob/main/providers/linode/flags.go)
- [Scaleway](https://github.com/crowci/autoscaler/blob/main/providers/scaleway/flags.go)
- [Vultr](https://github.com/crowci/autoscaler/blob/main/providers/vultr/flags.go)

/// tip
The links above show all available flags/env vars for the specific providers.
///

More providers which provide a golang SDK can be added - contributions welcome!

## Installation

The autoscaler should be installed alongside the _server_ instance as it listens for build triggers of it.
To connect to it, it needs to know the server address (`CROW_SERVER`) and an API token (`CROW_TOKEN`).
The token is used to add and remove agents from the server and must be from an admin user.

```yaml hl_lines="8 9"
[...]
  crow-autoscaler:
    image: ghcr.io/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
```

## Configuration

Next, the autoscaler configuration must be set.
You need to define how the scaling should work, i.e. how many servers are allowed to be provisioned and how many (if any) should be running at all times.

Often, allowing only one powerful server is enough, as this one will be able to process many pipelines in parallel (if the chosen instance type has enough resources).

Similar, `CROW_WORKFLOWS_PER_AGENT` defines how many workflows can be processed in parallel for each agent (each server has its own unique agent).
A good value for this setting depends on the resources of the server and the jobs which run on it, hence it is not possible to give a general recommendation that would fit all use cases.

/// tip
Monitor the resource usage of the first builds and see how many resources are needed.
Then adjust the autoscaler settings accordingly.
///

```yaml hl_lines="10 11 12"
[...]
  crow-autoscaler:
    image: ghcr.io/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_MIN_AGENTS=0
      - CROW_MAX_AGENTS=1
      - CROW_WORKFLOWS_PER_AGENT=5
```

### GRPC connection between agent and server

The next required step is to provide the GRPC connection information which the agent is going to use to connect to the server.
Remember, the agent will be running on a standalone server and must be able to connect to the crow _server_ instance over a public connection.
To do so securely, a SSL-backed connection is required.
This requires the server which is running the crow server instance to listen for incoming grpc requests.

To achieve this, two components are required:

- A reverse proxy listening on a https connection
- Forwarding of incoming requests to the grpc port of the crow server instance

We'll discuss how this can be done further below.

Coming back to the settings which need to be passed to the autoscaler.
Setting `CROW_GRPC_SECURE` is telling the upcoming agent to use SSL for all GRPC-based connections (by default it does not as usually the agent is on the same server as the crow server instance).
In addition, it needs to know where to connect to, which is why `CROW_GRPC_ADDR` is pointing to the public GRPC address of the server instance.

```yaml hl_lines="13 14"
[...]
  crow-autoscaler:
    image: ghcr.io/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_MIN_AGENTS=0
      - CROW_MAX_AGENTS=1
      - CROW_WORKFLOWS_PER_AGENT=5
      - CROW_GRPC_ADDR=grpc.your-crow-server.com # must be a public URL
      - CROW_GRPC_SECURE=true
```

### Cloud provider configuration

The last step is to tell the autoscaler which cloud provider to use.
This is done by setting `CROW_PROVIDER` and providing a token to authenticate.
The naming of this env var and the related provider-specific ones vary, depending on the features the provider offers.

```yaml hl_lines="15 16 17 18 19 20 21 22"
[...]
  crow-autoscaler:
    image: ghcr.io/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_MIN_AGENTS=0
      - CROW_MAX_AGENTS=1
      - CROW_WORKFLOWS_PER_AGENT=5
      - CROW_GRPC_ADDR=grpc.your-crow-server.com # must be a public URL
      - CROW_GRPC_SECURE=true
      - CROW_PROVIDER=hetznercloud
      - CROW_HETZNERCLOUD_API_TOKEN=${CROW_HETZNERCLOUD_API_TOKEN}
      - CROW_HETZNERCLOUD_LOCATION='fsn1' # Falkenstein
      - CROW_HETZNERCLOUD_SERVER_TYPE='cax41' # 16 cores, 32 GB RAM
      - CROW_HETZNERCLOUD_IMAGE='ubuntu-24.04'
      - CROW_HETZNERCLOUD_NETWORKS='<network name>'
      - CROW_HETZNERCLOUD_SSH_KEYS='<key name>'
      - CROW_HETZNERCLOUD_FIREWALLS='<firewall name>'
```

### Agent timeout & removal

There are two types of timeouts which can be set for the autoscaler

- `CROW_AGENT_IDLE_TIMEOUT` defines how long an agent is allowed to be idle before it is removed.
- `CROW_AGENT_SERVER_CONNECTION_TIMEOUT` defines how long an agent is kept alive after the last successful connection to the server has been established.

The first option is responsible for how long a server is being kept alive after the last build has been processed.

The second one is primarily a safety fallback for agents which are not able to communicate with the server anymore, for whatever reason (most likely due to a configuration issue).
In this case, the autoscaler will recognize this and shut down the instance to avoid infinite reconnection tries.

```yaml hl_lines="23 24"
[...]
  crow-autoscaler:
    image: ghcr.io/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_MIN_AGENTS=0
      - CROW_MAX_AGENTS=1
      - CROW_WORKFLOWS_PER_AGENT=5
      - CROW_GRPC_ADDR=grpc.your-crow-server.com # must be a public URL
      - CROW_GRPC_SECURE=true
      - CROW_PROVIDER=hetznercloud
      - CROW_HETZNERCLOUD_API_TOKEN=${CROW_HETZNERCLOUD_API_TOKEN}
      - CROW_HETZNERCLOUD_LOCATION='fsn1' # Falkenstein
      - CROW_HETZNERCLOUD_SERVER_TYPE='cax41' # 16 cores, 32 GB RAM
      - CROW_HETZNERCLOUD_IMAGE='ubuntu-24.04'
      - CROW_HETZNERCLOUD_NETWORKS='<network name>'
      - CROW_HETZNERCLOUD_SSH_KEYS='<key name>'
      - CROW_HETZNERCLOUD_FIREWALLS='<firewall name>'
      - CROW_AGENT_IDLE_TIMEOUT=10m
      - CROW_AGENT_SERVER_CONNECTION_TIMEOUT=10m
```

### Generic agent configuration

Last, there is the option to set arbitrary agent env vars via `CROW_AGENT_ENV`.
This can be helpful to control logging and other agent-specific settings.
The values must be passed as a comma-separated list:

```yaml
CROW_AGENT_ENV: CROW_HEALTHCHECK=false,CROW_LOG_LEVEL=debug
```

### Combining static and autoscaled agents

/// info
A "static" agent is an agent which was **not** provisioned by Crow Autoscaler but through a different deployment.
It is always 24/7 for new builds and is the exact opposite of an autoscaling agent which is only available if task are to be processed and decommissioned afterwards.
///

In specific cases a combination of static and autoscaled agents can make sense and improve the overall capabilities of your CI environment.
One use case can be if there are many builds with low resource requirements.
In this case, it is not efficient to wait for the autoscaler to spin up a powerful machine which then processes these builds in a few seconds.

In such a situation, a static agent on a small host (e.g. next to the Crow CI server and autoscaler) can help to process such builds.
The autoscaler will then only kick in once this agent cannot process these builds.

To achieve this, there must a heuristic to decide if the static agent can process these builds in question.
Ideally this would be a mix of a dynamic rule based on resource requests and a static ones based on manual filters.
Currently, only the latter is available through "agent labels" (`CROW_AGENT_LABELS`).
This options allows to set filters which only process specific builds that match these.
For example, if the mentioned builds in question only occur within one repository, the agent can be passed the following label through the env var `CROW_AGENT_LABELS`:

```yaml
CROW_AGENT_LABELS: 'repo=<owner>/<repo>'
```

This configuration ensures that only builds from this specific repository are processed.[^1]

When a Crow CI autoscaler instance is active, it first checks whether an existing agent registered with the Crow CI server can handle a pending task.
If no suitable agent is available, the autoscaler will launch a new agent.
If an agent is available, no action is taken, and the build is handled by the static agent.

### GRPC proxy configuration

As mentioned above, the GRPC connection between the agent and the server needs to be secured.
This is done by setting up a reverse proxy which listens on a https connection and forwards incoming requests to the grpc port of the crow server instance.

In the following examples for different reverse proxies are shown.
These are only minimal examples and you usually want to set additional headers and other options to secure the connection further.

/// note
The SSL certificate can be created like any other certificate for public servers.
Let's Encrypt is a good choice for this.
///

#### Nginx

```nginx
server {
    listen 443 ssl http2;
    server_name grpc.your-crow-server.com;

    ssl_certificate /etc/ssl/certs/your-crow-server.com.crt;
    ssl_certificate_key /etc/ssl/private/your-crow-server.com.key;

    location / {
        grpc_pass grpc://crow-server:9000;
    }
}
```

#### Caddy

```nginx
grpc.your-crow-server.com {
    reverse_proxy grpc://crow-server:9000
    tls /etc/ssl/certs/your-crow-server.com.crt /etc/ssl/private/your-crow-server.com.key
}
```

#### Traefik

```yaml
http:
  routers:
    grpc:
      rule: Host(`grpc.your-crow-server.com`)
      service: crow-server
      tls:
        certResolver: your-crow-server.com
  services:
    crow-server:
      loadBalancer:
        servers:
          - url: http://crow-server:9000
```

[^1]: Multiple labels can be passed, even of the same type.
