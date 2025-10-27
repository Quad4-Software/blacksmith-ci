## Server

### ADMIN

- Name: `CROW_ADMIN`
- Description: List of admin users.
- Default: none

---

### AGENT_SECRET

- Name: `CROW_AGENT_SECRET`
- Description: Server-agent shared password.
- Default: none

---

### AUTHENTICATE_PUBLIC_REPOS

- Name: `CROW_AUTHENTICATE_PUBLIC_REPOS`
- Description: Always use authentication to clone repositories even if they are public. Needed if the SCM requires to always authenticate as used by many companies.
- Default: `false`

---

### BACKEND_HTTP_PROXY

- Name: `CROW_BACKEND_HTTP_PROXY`
- Description: If set, pass the environment variable down as "HTTP_PROXY" to steps.
- Default: none

---

### BACKEND_HTTPS_PROXY

- Name: `CROW_BACKEND_HTTPS_PROXY`
- Description: If set, pass the environment variable down as "HTTPS_PROXY" to steps.
- Default: none

---

### BACKEND_NO_PROXY

- Name: `CROW_BACKEND_NO_PROXY`
- Description: If set, pass the environment variable down as "NO_PROXY" to steps.
- Default: none

---

### CONFIG_SERVICE_ENDPOINT

- Name: `CROW_CONFIG_SERVICE_ENDPOINT`
- Description: URL used for calling configuration service endpoint.
- Default: none

---

### CONTAINER_NAME_SCHEME

- Name: `CROW_CONTAINER_NAME_SCHEME`
- Description: Determines the naming scheme type for containers. Possible values are `descriptive` and `hash`.
  The `descriptive` scheme uses format `<owner>-<repo>-<pipeline>-<workflow>-<stepname>-<stepnumber>` for readability and uniqueness.
- Default: `descriptive`

---

### CRON_LOCK_TTL

- Name: `CROW_CRON_LOCK_TTL`
- Description: Time-to-live for the cron scheduler leader election lock in High Availability mode.
  If the leader node fails, other nodes must wait this duration before acquiring the lock.
- Default: `90s`

---

### CUSTOM_CSS_FILE

- Name: `CROW_CUSTOM_CSS_FILE`
- Description: File path for the server to serve a custom .CSS file, used for customizing the UI.
- Default: none

---

### CUSTOM_JS_FILE

- Name: `CROW_CUSTOM_JS_FILE`
- Description: File path for the server to serve a custom .JS file, used for customizing the UI.
- Default: none

---

### DATABASE_DATASOURCE

- Name: `CROW_DATABASE_DATASOURCE`
- Description: Database driver configuration string.
- Default: `/var/lib/crow/crow.sqlite` (or `crow.sqlite` if file exists)

---

### DATABASE_DRIVER

- Name: `CROW_DATABASE_DRIVER`
- Description: Database driver.
- Default: `sqlite3`

---

### DATABASE_LOG

- Name: `CROW_DATABASE_LOG`
- Description: Enable logging in database engine (currently xorm).
- Default: `false`

---

### DATABASE_LOG_SQL

- Name: `CROW_DATABASE_LOG_SQL`
- Description: Enable logging of SQL commands.
- Default: `false`

---

### DATABASE_MAX_CONNECTIONS

- Name: `CROW_DATABASE_MAX_CONNECTIONS`
- Description: Max connections xorm is allowed to create.
- Default: `100`

---

### DATABASE_IDLE_CONNECTIONS

- Name: `CROW_DATABASE_IDLE_CONNECTIONS`
- Description: Amount of connections xorm will hold open.
- Default: `2`

---

### DATABASE_CONNECTION_TIMEOUT

- Name: `CROW_DATABASE_CONNECTION_TIMEOUT`
- Description: Time an active connection is allowed to stay open.
- Default: `3s`

---

### DEFAULT_ALLOW_PULL_REQUESTS

- Name: `CROW_DEFAULT_ALLOW_PULL_REQUESTS`
- Description: The default value for allowing pull requests on a repo.
- Default: `true`

---

### DEFAULT_APPROVAL_MODE

- Name: `CROW_DEFAULT_APPROVAL_MODE`
- Description: The default setting for the approval mode on a repo. Possible values: `forks`, `pull_requests` or `all_events`.
- Default: `forks`

---

### DEFAULT_CANCEL_PREVIOUS_PIPELINE_EVENTS

- Name: `CROW_DEFAULT_CANCEL_PREVIOUS_PIPELINE_EVENTS`
- Description: List of event names that will be canceled when a new pipeline for the same context (tag, branch) is created.
- Default: `push,pull_request`

---

### DEFAULT_CLONE_PLUGIN

- Name: `CROW_DEFAULT_CLONE_PLUGIN`
- Description: The default docker image to be used when cloning the repo.
- Default: `git.coceci.org/crow/plugin-git:latest`

---

### DEFAULT_LOGS_KEEP_DURATION

- Name: `CROW_DEFAULT_LOGS_KEEP_DURATION`
- Description: The default time (as duration) for which pipeline logs of historic runs are preserved.
- Default: none

---

### DEFAULT_LOGS_PIPELINES_KEEP_MIN

- Name: `CROW_DEFAULT_LOGS_PIPELINES_KEEP_MIN`
- Description: The default amount of pipelines for which logs are preserved.
- Default: `90`

---

### DEFAULT_PIPELINE_TIMEOUT

- Name: `CROW_DEFAULT_PIPELINE_TIMEOUT`
- Description: The default time in minutes for a repo before a pipeline gets killed.
- Default: `60`

---

### DEFAULT_WORKFLOW_LABELS

- Name: `CROW_DEFAULT_WORKFLOW_LABELS`
- Description: The default label filter to set for workflows that has no label filter set. By default workflows will be allowed to run on any agent, if not specified in the workflow.
- Default: none

---

### DISABLE_USER_AGENT_REGISTRATION

- Name: `CROW_DISABLE_USER_AGENT_REGISTRATION`
- Description: Disable user registered agents.
- Default: `false`

---

### DISABLE_VERSION_CHECK

- Name: `CROW_DISABLE_VERSION_CHECK`
- Description: Disable version check in admin web UI.
- Default: `false`

---

### DOCKER_CONFIG

- Name: `CROW_DOCKER_CONFIG`
- Description: Docker configuration.
- Default: none

---

### ENABLE_HA

- Name: `CROW_ENABLE_HA`
- Description: Enable high availability mode with database-backed queue.
- Default: `false`

---

### ENABLE_SWAGGER

- Name: `CROW_ENABLE_SWAGGER`
- Description: Enable Swagger API documentation.
- Default: `true`

---

### ENCRYPTION_DISABLE_FLAG

- Name: `CROW_ENCRYPTION_DISABLE`
- Description: Flag to decrypt all encrypted data and disable encryption on server.
- Default: `false`

---

### ENCRYPTION_RAW_KEY

- Name: `CROW_ENCRYPTION_KEY`
- Description: Raw encryption key.
- Default: none

---

### ENCRYPTION_TINK_KEYSET_FILE

- Name: `CROW_ENCRYPTION_TINK_KEYSET_FILE`
- Description: Google tink AEAD-compatible keyset file to encrypt secrets in DB.
- Default: none

---

### ENVIRONMENT

- Name: `CROW_ENVIRONMENT`
- Description: Environment variables.
- Default: none

---

### FORGE_OAUTH_CLIENT

- Name: `CROW_FORGE_CLIENT`
- Description: OAuth2 client ID.
- Default: none

---

### FORGE_OAUTH_HOST

- Name: `CROW_EXPERT_FORGE_OAUTH_HOST`
- Description: Fully qualified public forge URL. Use it if your forge URL isn't a public URL. Format: `<scheme>://<host>[/<prefix path>]`
- Default: none

---

### FORGE_OAUTH_SECRET

- Name: `CROW_FORGE_SECRET`
- Description: OAuth2 client secret.
- Default: none

---

### FORGE_RETRY

- Name: `CROW_FORGE_RETRY`
- Description: How many retries of fetching the Crow configuration from a forge are done before we fail.
- Default: `3`

---

### FORGE_SKIP_VERIFY

- Name: `CROW_FORGE_SKIP_VERIFY`
- Description: Skip SSL verification.
- Default: `false`

---

### FORGE_TIMEOUT

- Name: `CROW_FORGE_TIMEOUT`
- Description: How many seconds before timeout when fetching the Crow configuration from a Forge.
- Default: `5s`

---

### FORGE_URL

- Name: `CROW_FORGE_URL`
- Description: URL of the forge.
- Default: none

---

### GRPC_ADDR

- Name: `CROW_GRPC_ADDR`
- Description: gRPC address.
- Default: `:9000`

---

### GRPC_SECRET

- Name: `CROW_GRPC_SECRET`
- Description: gRPC JWT secret.
- Default: `secret`

---

### HOST

- Name: `CROW_HOST`
- Description: Server fully qualified URL. Format: `<scheme>://<host>[/<prefix path>]`
- Default: none

---

### KEEPALIVE_MIN_TIME

- Name: `CROW_KEEPALIVE_MIN_TIME`
- Description: Server-side enforcement policy on the minimum amount of time a client should wait before sending a keepalive ping.
- Default: none

---

### MAINTENANCE_KUBERNETES_CLEANUP_ENABLED

- Name: `MAINTENANCE_KUBERNETES_CLEANUP_ENABLED`
- Description: Whether to enable Kubernetes resource cleanup.
- Default: `none` (`true` when `CROW_BACKEND=kubernetes`)

### MAINTENANCE_KUBERNETES_CLEANUP_AGE

- Name: `MAINTENANCE_KUBERNETES_CLEANUP_AGE`
- Description: The age threshold for Kubernetes resources to be cleaned up.
- Default: `168h`

---

### MAINTENANCE_KUBERNETES_CLEANUP_NAMESPACES

- Name: `MAINTENANCE_KUBERNETES_CLEANUP_NAMESPACES`
- Description: Comma-separated list of namespaces to clean up.
- Default: `none` (applies to current server namespace)

---

### MAINTENANCE_STALE_AGENT_CLEANUP_ENABLED

- Name: `CROW_MAINTENANCE_STALE_AGENT_CLEANUP_ENABLED`
- Description: Enable automatic stale agent cleanup.
- Default: `true`

---

### MAINTENANCE_STALE_AGENT_DURATION

- Name: `CROW_MAINTENANCE_STALE_AGENT_DURATION`
- Description: Duration after which agents are considered stale (e.g., 24h for 1 day).
- Default: `24h`

---

### MAINTENANCE_VACUUM_LOG_TABLE_ENABLED

- Name: `CROW_MAINTENANCE_VACUUM_LOG_TABLE_ENABLED`
- Description: Whether to enable vacuuming of the log table.
- Default: `true`

---

### MAINTENANCE_VACUUM_LOG_TABLE_SCHEDULE

- Name: `CROW_MAINTENANCE_VACUUM_LOG_TABLE_SCHEDULE`
- Description: Cron schedule for vacuuming the log table.
- Default: `0 5 * * *`

---

### MAX_PIPELINE_TIMEOUT

- Name: `CROW_MAX_PIPELINE_TIMEOUT`
- Description: The maximum time in minutes you can set in the repo settings before a pipeline gets killed.
- Default: `120`

---

### METRICS_SERVER_ADDR

- Name: `CROW_METRICS_SERVER_ADDR`
- Description: Metrics server address.
- Default: none

---

### MIGRATIONS_ALLOW_LONG

- Name: `CROW_MIGRATIONS_ALLOW_LONG`
- Description: Allow long-running migrations.
- Default: `false`

---

### NETWORK

- Name: `CROW_NETWORK`
- Description: Networks.
- Default: none

---

### OPEN

- Name: `CROW_OPEN`
- Description: Enable open user registration.
- Default: `false`

---

### ORGS

- Name: `CROW_ORGS`
- Description: List of approved organizations.
- Default: none

---

### PLUGINS_PRIVILEGED

- Name: `CROW_PLUGINS_PRIVILEGED`
- Description: Allow plugins to run in privileged mode. If environment variable is defined but empty there will be none.
- Default: none

---

### PLUGINS_PRIVILEGED_MATCH_TYPE

- Name: `CROW_PLUGINS_PRIVILEGED_MATCH_TYPE`
- Description: Type of match for privileged plugins: `semver`, `semver-range`, `regex`, or `exact`.
- Default: `semver`

---

### PLUGINS_TRUSTED_CLONE

- Name: `CROW_PLUGINS_TRUSTED_CLONE`
- Description: Plugins which are trusted to handle Git credentials in clone steps.
- Default: `git.coceci.org/crow/plugin-git:*`

---

### PROMETHEUS_AUTH_TOKEN

- Name: `CROW_PROMETHEUS_AUTH_TOKEN`
- Description: Token to secure Prometheus metrics endpoint.
- Default: none

---

### REPO_OWNERS

- Name: `CROW_REPO_OWNERS`
- Description: Repositories by those owners will be allowed to be used in Crow.
- Default: none

---

### SERVER_ADDR

- Name: `CROW_SERVER_ADDR`
- Description: Server address.
- Default: `:8000`

---

### SERVER_ADDR_TLS

- Name: `CROW_SERVER_ADDR_TLS`
- Description: Port HTTPS with TLS.
- Default: `:443`

---

### SERVER_CERT

- Name: `CROW_SERVER_CERT`
- Description: Server SSL cert path.
- Default: none

---

### SERVER_KEY

- Name: `CROW_SERVER_KEY`
- Description: Server SSL key path.
- Default: none

---

### SERVER_WEBHOOK_HOST

- Name: `CROW_EXPERT_WEBHOOK_HOST`
- Description: Fully qualified Crow server URL called by forge's webhooks. Format: `<scheme>://<host>[/<prefix path>]`. For experts only!
- Default: none

---

### SESSION_EXPIRES

- Name: `CROW_SESSION_EXPIRES`
- Description: Session expiration time.
- Default: `72h`

---

### STATUS_CONTEXT

- Name: `CROW_STATUS_CONTEXT`
- Description: Status context prefix.
- Default: `ci/crow`

---

### STATUS_CONTEXT_FORMAT

- Name: `CROW_STATUS_CONTEXT_FORMAT`
- Description: Status context format.
- Default: `{{ .context }}/{{ .event }}/{{ .workflow }}{{if not (eq .axis_id 0)}}/{{.axis_id}}{{end}}`

---

### VOLUME

- Name: `CROW_VOLUME`
- Description: Volumes.
- Default: none

---

### WWW_PROXY

- Name: `CROW_DEV_WWW_PROXY`
- Description: Serve the website by using a proxy (used for development).
- Default: none

---

### ADDON_FORGE

- Name: `CROW_ADDON_FORGE`
- Description: Path to forge addon executable.
- Default: none

---

### BITBUCKET

- Name: `CROW_BITBUCKET`
- Description: Bitbucket driver is enabled.
- Default: `false`

---

### BITBUCKET_DC

- Name: `CROW_BITBUCKET_DC`
- Description: Bitbucket DataCenter/Server driver is enabled.
- Default: `false`

---

### BITBUCKET_DC_GIT_USERNAME

- Name: `CROW_BITBUCKET_DC_GIT_USERNAME`
- Description: Bitbucket DataCenter/Server service account username.
- Default: none

---

### BITBUCKET_DC_GIT_PASSWORD

- Name: `CROW_BITBUCKET_DC_GIT_PASSWORD`
- Description: Bitbucket DataCenter/Server service account password.
- Default: none

---

### FORGEJO

- Name: `CROW_FORGEJO`
- Description: Forgejo driver is enabled.
- Default: `false`

---

### GITEA

- Name: `CROW_GITEA`
- Description: Gitea driver is enabled.
- Default: `false`

---

### GITHUB

- Name: `CROW_GITHUB`
- Description: GitHub driver is enabled.
- Default: `false`

---

### GITHUB_MERGE_REF

- Name: `CROW_GITHUB_MERGE_REF`
- Description: GitHub pull requests use merge ref.
- Default: `true`

---

### GITHUB_PUBLIC_ONLY

- Name: `CROW_GITHUB_PUBLIC_ONLY`
- Description: GitHub tokens should only get access to public repos.
- Default: `false`

---

### GITLAB

- Name: `CROW_GITLAB`
- Description: GitLab driver is enabled.
- Default: `false`

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

### LOG_STORE

- Name: `CROW_LOG_STORE`
- Description: Log store to use (`database` or `file`).
- Default: `database`

---

### LOG_STORE_FILE_PATH

- Name: `CROW_LOG_STORE_FILE_PATH`
- Description: Directory used for file based log storage.
- Default: none

---

### QUEUE_LOCK_TTL

- Name: `CROW_QUEUE_LOCK_TTL`
- Description: Time-to-live for the queue processing leader election lock in High Availability mode.
  If the leader node fails, other nodes must wait this duration before acquiring the lock.
- Default: `30s`

## Agent

### AGENT_CONFIG_FILE

- Name: `CROW_AGENT_CONFIG_FILE`
- Description: Agent config file path. If set empty the agent will be stateless and unregister on termination.
- Default: `/etc/crow/agent.conf`

---

### AGENT_LABELS

- Name: `CROW_AGENT_LABELS`
- Description: Configures custom labels for the agent, to enable workflow filtering. Accepts a list of key-value pairs like `key=value,second-key=*`.
  Agents provide three additional labels `platform=os/arch`, `hostname=my-agent` and `repo=*` which can be overwritten if needed.
- Default: none

---

### AGENT_PRIORITY

- Name: `CROW_AGENT_PRIORITY`
- Description: Agent priority for task assignment (higher values get tasks first).
- Default: `0`

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
- Description: Crow backend to use. Possible values are `auto-detect`, `docker`, `local` or `kubernetes`.
- Default: `auto-detect`

---

### BACKEND_DOCKER_NETWORK

- Name: `CROW_BACKEND_DOCKER_NETWORK`
- Description: Docker network which will be attached to all pipeline steps. Careful: this allows containers of different steps to communicate with each other.
- Default: none

---

### BACKEND_DOCKER_ENABLED_IPV6

- Name: `CROW_BACKEND_DOCKER_ENABLED_IPV6`
- Description: Whether to enable IPv6 support in Docker containers. Requires the Docker daemon to be configured with IPv6 support.
- Default: `false`

---

### BACKEND_DOCKER_VOLUMES

- Name: `CROW_BACKEND_DOCKER_VOLUMES`
- Description: Additional volumes to mount in Docker containers.
- Default: none

---

### BACKEND_DOCKER_LIMIT_MEM_SWAP

- Name: `CROW_BACKEND_DOCKER_LIMIT_MEM_SWAP`
- Description: Whether to limit memory swap usage in Docker containers. No limit is applied if set to `0`.
- Default: `0`

---

### BACKEND_DOCKER_LIMIT_MEM

- Name: `CROW_BACKEND_DOCKER_LIMIT_MEM`
- Description: Whether to limit memory usage in Docker containers. No limit is applied if set to `0`.
- Default: `0`

---

### BACKEND_DOCKER_LIMIT_SHM_SIZE

- Name: `CROW_BACKEND_DOCKER_LIMIT_SHM_SIZE`
- Description: Whether to limit shared memory size in Docker containers. No limit is applied if set to `0`.
- Default: `0`

---

### BACKEND_DOCKER_LIMIT_CPU_QUOTA

- Name: `CROW_BACKEND_DOCKER_LIMIT_CPU_QUOTA`
- Description: Microseconds per CPU quota before throttling takes place. No limit is applied if set to `0`.
- Default: `0`

---

### BACKEND_DOCKER_LIMIT_CPU_SHARES

- Name: `CROW_BACKEND_DOCKER_LIMIT_CPU_SHARES`
- Description: CPU shares (relative weight) for Docker containers. No limit is applied if set to `0`.
- Default: `0`

---

### BACKEND_DOCKER_LIMIT_CPU_SET

- Name: `CROW_BACKEND_DOCKER_LIMIT_CPU_SET`
- Description: Comma-separated list of specific CPU cores the pipelines are limited to. No limit is applied if set to `0`.
- Default: none

### BACKEND_K8S_NAMESPACE

- Name: `CROW_BACKEND_K8S_NAMESPACE`
- Description: The k8s namespace to execute pipelines in
- Default: `crow`

---

### BACKEND_K8S_NAMESPACE_PER_ORGANIZATION

- Name: `CROW_BACKEND_K8S_NAMESPACE_PER_ORGANIZATION`
- Description: Enables namespace isolation per organization.
  When enabled, each organization gets its own dedicated Kubernetes namespace for improved security and resource isolation.
  The name format follows `{CROW_BACKEND_K8S_NAMESPACE}-{organization-id}`.
  Namespaces are created automatically, but not removed when organizations are removed from the Crow instance.
- Default: `false`

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

### BACKEND_K8S_POD_TOLERATIONS

- Name: `CROW_BACKEND_K8S_POD_TOLERATIONS`
- Description: Additional tolerations to apply to worker Pods.
  Must be a YAML object, e.g. `[{"effect":"NoSchedule","key":"jobs","operator":"Exists"}]`.
- Default: none

---

### BACKEND_K8S_POD_TOLERATIONS_ALLOW_FROM_STEP

- Name: `CROW_BACKEND_K8S_POD_TOLERATIONS_ALLOW_FROM_STEP`
- Description: Determines if Pod tolerations can be defined from a step's backend options.
- Default: `true`

---

### BACKEND_K8S_PRIORITY_CLASS

- Name: `CROW_BACKEND_K8S_PRIORITY_CLASS`
- Description: Which [Kubernetes PriorityClass](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/priority-class-v1/) to assign to created job pods.
- Default: none, which will use the default priority class configured in Kubernetes

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

### PLUGINS_PRIVILEGED

- Name: `CROW_PLUGINS_PRIVILEGED`
- Description: List of plugin images allowed to run in privileged mode. Matching is controlled by `CROW_PLUGINS_PRIVILEGED_MATCH_TYPE`.
- Default: none

---

### PLUGINS_PRIVILEGED_MATCH_TYPE

- Name: `CROW_PLUGINS_PRIVILEGED_MATCH_TYPE`
- Description: Type of match for privileged plugins. Possible values are `semver`, `semver-range`, `regex`, or `exact`. Controls how entries in `CROW_PLUGINS_PRIVILEGED` are matched against plugin images.
- Default: `semver`

[^1]:
    The provider-specific settings are documented in their respective source files to keep this page tidy.
    All links can be found [in the Autoscaler documentation](autoscaler.md).
