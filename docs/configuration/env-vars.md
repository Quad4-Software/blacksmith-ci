## Server

### BACKEND_K8S_NAMESPACE

- Name: `CROW_BACKEND_K8S_NAMESPACE`
- Description: The k8s namespace to execute pipelines in
- Default: `crow`

---

### BACKEND_K8S_POD_ANNOTATIONS

- Name: `CROW_BACKEND_K8S_POD_ANNOTATIONS`
- Description: Additional annotations to apply to worker Pods.
  Must be a YAML object, e.g.<br> `{"example.com/test-annotation":"test-value"}`
- Default: `crow`

---

### BACKEND_K8S_POD_LABELS_ALLOW_FROM_STEP

- Name: `CROW_BACKEND_K8S_POD_LABELS_ALLOW_FROM_STEP`
- Description: Determines if Pod annotations can be defined from a step's backend options.
- Default: `false`

---

### BACKEND_K8S_POD_LABELS

- Name: `CROW_BACKEND_K8S_POD_LABELS`
- Description: Additional labels to apply to worker Pods.
  Must be a YAML object, e.g. `{"example.com/test-label":"test-value"}`
- Default: none

---

### BACKEND_K8S_POD_NODE_SELECTOR

- Name: `CROW_BACKEND_K8S_POD_NODE_SELECTOR`
- Description: Additional node selector to apply to worker pods.
  Must be a YAML object, e.g. `{"topology.kubernetes.io/region":"eu-central-1"}`
- Default: none

---

### BACKEND_K8S_PULL_SECRET_NAMES

- Name: `CROW_BACKEND_K8S_PULL_SECRET_NAMES`
- Description: Secret names to pull images from private repositories.
- Default: none

---

### BACKEND_K8S_SECCTX_NONROOT

- Name: `CROW_BACKEND_K8S_SECCTX_NONROOT`
- Description: Whether containers must be run as a non-root user.
- Default: `false`

---

### BACKEND_K8S_STORAGE_CLASS

- Name: `CROW_BACKEND_K8S_STORAGE_CLASS`
- Description: The storage class to use for the temporary pipeline volume.
- Default:

---

### BACKEND_K8S_STORAGE_RWX

- Name: `CROW_BACKEND_K8S_STORAGE_RWX`
- Description: Whether a RWX should be used for the temporary pipeline volume.
  If false, RWO is used instead.
- Default: `true`

---

### BACKEND_K8S_VOLUME_SIZE

- Name: `CROW_BACKEND_K8S_VOLUME_SIZE`
- Description: The volume size of the temporary pipeline volume.
- Default: `10G`

---

### BACKEND_LOCAL_TEMP_DIR

- Name: `CROW_BACKEND_LOCAL_TEMP_DIR`
- Description: Directory in which pipelines are executed
- Default: `$TMPDIR`

---

### LOG_LEVEL

- Name: `CROW_LOG_LEVEL`
- Description: Logging level. Possible values are `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, and `disabled`.
- Default: none

---

### LOG_FILE

- Name: `CROW_LOG_FILE`
- Description: Output destination for logs. `stdout` and `stderr` can be used as special keywords.
- Default: `stderr`

---

## Agent

### AGENT_CONFIG_FILE

- Name: `CROW_AGENT_CONFIG_FILE`
- Description: Filepath containing agent config, e.g. `/etc/woodpecker/agent.conf`
- Default: none

---

### AGENT_LABELS

- Name: `CROW_AGENT_LABELS`
- Description: Configures custom labels for the agent, to enable workflow filtering. Accepts a list of key-value pairs like `key=value,second-key=*`.
  Agents provide three additional labels `platform=os/arch`, `hostname=my-agent` and `repo=*` which can be overwritten if needed.
- Default: none

---

### AGENT_SECRET_FILE

- Name: `CROW_AGENT_SECRET_FILE`
- Description: Filepath containing the agent secret, e.g. `/etc/woodpecker/agent-secret.conf`
- Default: none

---

### AGENT_SECRET

- Name: `CROW_AGENT_SECRET`
- Description: A shared secret used by server and agents to authenticate communication.
  A secret can be generated via `openssl rand -hex 32`.
- Default: none

---

### BACKEND

- Name: `CROW_BACKEND`
- Description: Woodpecker backend to use. Possible values are `auto-detect`, `docker`, `local` or `kubernetes`.
- Default: `auto-detect`

---

### CONNECT_RETRY_COUNT

- Name: `CROW_CONNECT_RETRY_COUNT`
- Description: Number of times agent retries to connect to the server.
- Default: `5`

---

### CONNECT_RETRY_DELAY

- Name: `CROW_CONNECT_RETRY_DELAY`
- Description: Delay between agent connection retries to the server.
- Default: `2s`

---

### DEBUG_NOCOLOR

- Name: `CROW_DEBUG_NOCOLOR`
- Description: Disable colored debug output.
- Default: `true`

---

### DEBUG_PRETTY

- Name: `CROW_DEBUG_PRETTY`
- Description: Enable pretty-printed debug output.
- Default: `false`

---

### GRPC_SECURE

- Name: `CROW_GRPC_SECURE`
- Description: Whether the connection to `CROW_SERVER` should be made via SSL.
- Default: `false`

---

### GRPC_VERIFY

- Name: `CROW_GRPC_VERIFY`
- Description: Whether the `grpc` server certificate should be verified. Only valid when `CROW_GRPC_SECURE=true`.
- Default: `true`

---

### HEALTHCHECK_ADDR

- Name: `CROW_HEALTHCHECK_ADDR`
- Description: Healthcheck endpoint address.
- Default: `:3000`

---

### HEALTHCHECK

- Name: `CROW_HEALTHCHECK`
- Description: Enable healthcheck endpoint.
- Default: `true`

---

### HOSTNAME

- Name: `CROW_HOSTNAME`
- Description: Agent hostname
- Default: none

---

### KEEPALIVE_TIME

- Name: `CROW_KEEPALIVE_TIME`
- Description: With no activity after this duration, the agent pings the server to check if the transport is still alive.
- Default: none

---

### KEEPALIVE_TIMEOUT

- Name: `CROW_KEEPALIVE_TIMEOUT`
- Description: After pinging for a keepalive check, the agent waits for this duration before closing unresponsive connections.
- Default: `20s`

---

<!-- markdownlint-disable MD024 -->
### LOG_LEVEL

- Name: `CROW_LOG_LEVEL`
- Description: Logging level. Possible values are `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`, and `disabled`.
- Default: none

---

### MAX_WORKFLOWS

- Name: `CROW_MAX_WORKFLOWS`
- Description: Number of parallel workflows.
- Default: `1`

---

### SERVER

- Name: `CROW_SERVER`
- Description: gRPC address of the server.
- Default: `localhost:9000`

---

### USERNAME

- Name: `CROW_USERNAME`
- Description: gRPC username.
- Default: `x-oauth-basic`

---

<!-- markdownlint-disable MD024 -->

### LOG_FILE

- Name: `CROW_LOG_FILE`
- Description: Output destination for logs. `stdout` and `stderr` can be used as special keywords.
- Default: `stderr`

## Autoscaler[^1]

### LOG_LEVEL

- Name: `CROW_LOG_LEVEL`
- Description: default log level
- Default: `info`

---

### RECONCILIATION_INTERVAL

- Name: `CROW_RECONCILIATION_INTERVAL`
- Description: interval at which the autoscaler will reconsider a scale up/down
- Default: `1m`

---

### POOL_ID

- Name: `CROW_POOL_ID`
- Description: id of the autoscaler pool
- Default: `1`

---

### AGENT_INACTIVITY_TIMEOUT

- Name: `CROW_AGENT_INACTIVITY_TIMEOUT`
- Description: time an agent is allowed to be inactive before it can be terminated as duration string like `2h45m`
- Default: `10m`
- Synonyms:
    - `CROW_AGENT_ALLOWED_STARTUP_TIME`
    - `CROW_AGENT_SERVER_CONNECTION_TIMEOUT`

---

### AGENT_IDLE_TIMEOUT

- Name: `CROW_AGENT_IDLE_TIMEOUT`
- Description: time an agent is allowed to be idle before it can be terminated as duration string like `2h45m`
- Default: `10m`

[^1]: The provider-specific settings are documented in their respective source files to keep this page tidy.
  All links can be found [here](autoscaler.md).
