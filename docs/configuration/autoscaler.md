## Preface: What is the 'Crow CI Autoscaler'?

The [Crow CI Autoscaler](https://github.com/crowci/autoscaler) is a standalone application that spins up servers in a configured cloud provider to execute Crow CI pipelines.
Once the VM is ready, it creates a Crow CI agent that picks up pending pipeline jobs.

If no new builds are in the queue, the Autoscaler waits for a configurable period (default: 10 minutes) before removing the server.

/// tip
Depending on your cloud provider's minimum billing cycle, you may want to increase this value.

For example, some providers bill in one-hour increments, so removing a server after 10 minutes may not make sense if it might pick up a job after 40 minutes.
Once spawned, you're billed for the entire hour anyway.
///

The app allows you to utilize powerful servers on-demand and terminate them immediately after use.
This minimizes costs and eliminates the expense of maintaining large idle instances when no builds are running.

The Crow CI Autoscaler currently works with the following cloud providers:

- [AWS](https://github.com/crowci/autoscaler/blob/main/providers/aws/flags.go)
- [Hetzner Cloud](https://github.com/crowci/autoscaler/blob/main/providers/hetzner/flags.go)
- [Linode](https://github.com/crowci/autoscaler/blob/main/providers/linode/flags.go)
- [Scaleway](https://github.com/crowci/autoscaler/blob/main/providers/scaleway/flags.go)
- [Vultr](https://github.com/crowci/autoscaler/blob/main/providers/vultr/flags.go)

/// tip
The links above show all available flags/env vars for each provider.
///

Additional providers with a Golang SDK can be added - contributions welcome!

## Installation

The autoscaler should be installed alongside the _server_ instance as it listens for build triggers from it.
To connect, it needs to know the server address (`CROW_SERVER`), a token to register against the server (`CROW_AUTOSCALER_TOKEN`), and an admin API token (`CROW_TOKEN`) to execute actions against the API.
The token is used to add and remove agents from the server and must belong to an admin user.

```yaml hl_lines="8 9 10"
[...]
  crow-autoscaler:
    image: codeberg.org/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_AUTOSCALER_TOKEN=${CROW_AUTOSCALER_TOKEN} # since autoscaler v1.*
```

## Configuration

Next, you must configure the autoscaler settings.
You need to define how scaling should work, i.e., how many servers are allowed to be provisioned and how many (if any) should be running at all times.

Often, a single powerful server is sufficient, as it can process many pipelines in parallel (if the chosen instance type has enough resources).

Similarly, `CROW_WORKFLOWS_PER_AGENT` defines how many workflows can be processed in parallel for each agent (each server has its own unique agent).
A good value for this setting depends on the server's resources and the jobs running on it, so it's not possible to give a general recommendation that fits all use cases.

/// tip
Monitor the resource usage during initial builds to determine how many resources are needed.
Then adjust the autoscaler settings accordingly.
///

```yaml hl_lines="10 11 12"
[...]
  crow-autoscaler:
    image: codeberg.org/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_AUTOSCALER_TOKEN=${CROW_AUTOSCALER_TOKEN}
      - CROW_MIN_AGENTS=0
      - CROW_MAX_AGENTS=1
      - CROW_WORKFLOWS_PER_AGENT=5
```

### GRPC connection between agent and server

The next required step is to provide the GRPC connection information that the agent will use to connect to the server.
Remember, the agent will be running on a standalone server and must be able to connect to the Crow _server_ instance over a public connection.
To do this securely, an SSL-backed connection is required.
This requires the server running the Crow server instance to listen for incoming GRPC requests.

To achieve this, two components are required:

- A reverse proxy listening on an HTTPS connection
- Forwarding of incoming requests to the GRPC port of the Crow server instance

We'll discuss how this can be done further below.

Returning to the settings that need to be passed to the autoscaler:
Setting `CROW_GRPC_SECURE` tells the agent to use SSL for all GRPC-based connections (by default it doesn't, as typically the agent is on the same server as the Crow server instance).
Additionally, it needs to know where to connect, which is why `CROW_GRPC_ADDR` points to the public GRPC address of the server instance.

```yaml hl_lines="13 14"
[...]
  crow-autoscaler:
    image: codeberg.org/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_AUTOSCALER_TOKEN=${CROW_AUTOSCALER_TOKEN}
      - CROW_MIN_AGENTS=0
      - CROW_MAX_AGENTS=1
      - CROW_WORKFLOWS_PER_AGENT=5
      - CROW_GRPC_ADDR=grpc.your-crow-server.com # must be a public URL
      - CROW_GRPC_SECURE=true
```

### Cloud provider configuration

The last step is to specify which cloud provider to use.
This is done by setting `CROW_PROVIDER` and providing an authentication token.
The naming of this environment variable and related provider-specific ones varies depending on the features the provider offers.

```yaml hl_lines="15 16 17 18 19 20 21 22"
[...]
  crow-autoscaler:
    image: codeberg.org/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_AUTOSCALER_TOKEN=${CROW_AUTOSCALER_TOKEN}
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

There are two types of timeouts that can be configured for the autoscaler:

- `CROW_AGENT_IDLE_TIMEOUT` defines how long an agent is allowed to be idle before it is removed.
- `CROW_AGENT_SERVER_CONNECTION_TIMEOUT` defines how long an agent is kept alive after the last successful connection to the server.

The first option controls how long a server is kept alive after the last build has been processed.

The second one is primarily a safety fallback for agents that can no longer communicate with the server for any reason (most likely due to a configuration issue).
In this case, the autoscaler will recognize this and shut down the instance to avoid infinite reconnection attempts.

```yaml hl_lines="23 24"
[...]
  crow-autoscaler:
    image: codeberg.org/crowci/crow-autoscaler:<tag>
    restart: always
    depends_on:
      - crow-server
    environment:
      - CROW_SERVER=crow-server:9000 # can also be the public URL (https://)
      - CROW_TOKEN=${CROW_TOKEN}
      - CROW_AUTOSCALER_TOKEN=${CROW_AUTOSCALER_TOKEN}
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

Finally, you can set arbitrary agent environment variables via `CROW_AGENT_ENV`.
This can be helpful for controlling logging and other agent-specific settings.
The values must be passed as a comma-separated list:

```yaml
CROW_AGENT_ENV: CROW_HEALTHCHECK=false,CROW_LOG_LEVEL=debug
```

### Combining static and autoscaled agents

/// info
A "static" agent is an agent that was **not** provisioned by Crow Autoscaler but deployed separately.
It is available 24/7 for new builds and is the exact opposite of an autoscaled agent, which is only available when tasks need to be processed and is decommissioned afterwards.
///

In specific cases, a combination of static and autoscaled agents can make sense and improve the overall capabilities of your CI environment.
One use case is when there are many builds with low resource requirements.
In this case, it's inefficient to wait for the autoscaler to spin up a powerful machine that processes these builds in just a few seconds.

In such situations, a static agent on a small host (e.g., alongside the Crow CI server and autoscaler) can help process these builds.
The autoscaler will then only activate when this agent cannot handle the workload.

To achieve this, there must be a heuristic to determine whether the static agent can process the builds in question.
Ideally, this would be a mix of dynamic rules based on resource requests and static ones based on manual filters.
Currently, only the latter is available through "agent labels" (`CROW_AGENT_LABELS`).
This option allows you to set filters that only process specific builds matching them.
For example, if the builds in question only occur within one repository, the agent can be configured with the following label via the `CROW_AGENT_LABELS` environment variable:

```yaml
CROW_AGENT_LABELS: 'repo=<owner>/<repo>'
```

This configuration ensures that only builds from this specific repository are processed.[^1]

When a Crow CI autoscaler instance is active, it first checks whether an existing agent registered with the Crow CI server can handle a pending task.
If no suitable agent is available, the autoscaler will launch a new agent.
If an agent is available, no action is taken, and the build is handled by the static agent.

### GRPC proxy configuration

As mentioned above, the GRPC connection between the agent and server needs to be secured.
This is done by setting up a reverse proxy that listens on an HTTPS connection and forwards incoming requests to the GRPC port of the Crow server instance.

Below are examples for different reverse proxies.
These are minimal examples and you should typically set additional headers and other options to further secure the connection.

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
