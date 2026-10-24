-- Baseline schema for Crow CI (PostgreSQL)
-- Generated from server/model/ bun struct tags

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    forge_id INTEGER,
    forge_remote_id TEXT,
    primary_forge_id INTEGER,
    login TEXT UNIQUE,
    access_token TEXT,
    refresh_token TEXT,
    expiry INTEGER,
    email VARCHAR(500),
    avatar VARCHAR(500),
    admin BOOLEAN,
    hash VARCHAR(500) UNIQUE,
    org_id INTEGER,
    description TEXT,
    timezone VARCHAR(100),
    roles TEXT
);

CREATE TABLE IF NOT EXISTS user_forges (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    forge_id INTEGER NOT NULL,
    forge_remote_id TEXT,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    access_token TEXT,
    refresh_token TEXT,
    expiry INTEGER,
    forge_login VARCHAR(250),
    forge_email VARCHAR(500),
    forge_avatar VARCHAR(500),
    linked_at INTEGER NOT NULL,
    last_used INTEGER,
    UNIQUE(forge_id, forge_remote_id)
);

CREATE TABLE IF NOT EXISTS user_identities (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    provider VARCHAR(50) NOT NULL,
    issuer VARCHAR(255) NOT NULL DEFAULT '',
    subject VARCHAR(255) NOT NULL,
    email VARCHAR(500),
    username VARCHAR(250),
    idp_groups TEXT,
    linked_at INTEGER NOT NULL,
    last_login INTEGER,
    UNIQUE(provider, issuer, subject)
);

CREATE TABLE IF NOT EXISTS pipeline_watches (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    repo_id BIGINT NOT NULL,
    watch_event VARCHAR(50) NOT NULL,
    watch_target VARCHAR(255) NOT NULL,
    in_app BOOLEAN NOT NULL,
    email_enabled BOOLEAN NOT NULL,
    email_events TEXT,
    last_read_pipeline BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT,
    updated_at BIGINT,
    UNIQUE(user_id, repo_id, watch_event, watch_target)
);

CREATE TABLE IF NOT EXISTS watch_workflow_states (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL,
    watch_event VARCHAR(50) NOT NULL,
    watch_target VARCHAR(255) NOT NULL,
    workflow_name VARCHAR(255) NOT NULL,
    failing BOOLEAN NOT NULL,
    pipeline_id BIGINT NOT NULL,
    pipeline_number BIGINT NOT NULL,
    runs_since_seen BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT,
    UNIQUE(repo_id, watch_event, watch_target, workflow_name)
);

CREATE TABLE IF NOT EXISTS build_caches (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL,
    scope VARCHAR(255) NOT NULL,
    cache_key VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    committed BOOLEAN NOT NULL DEFAULT false,
    created BIGINT NOT NULL DEFAULT 0,
    last_used BIGINT NOT NULL DEFAULT 0,
    bucket_id BIGINT NOT NULL DEFAULT 0,
    UNIQUE(repo_id, scope, cache_key)
);

CREATE TABLE IF NOT EXISTS build_cache_settings (
    org_id BIGINT PRIMARY KEY,
    max_size BIGINT NOT NULL DEFAULT 0,
    max_egress BIGINT NOT NULL DEFAULT 0,
    ttl BIGINT NOT NULL DEFAULT 0,
    updated BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS build_cache_buckets (
    id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL,
    endpoint VARCHAR(255) NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    region VARCHAR(255),
    prefix VARCHAR(255),
    access_key VARCHAR(255),
    secret_key TEXT,
    use_ssl BOOLEAN NOT NULL DEFAULT true,
    path_style BOOLEAN NOT NULL DEFAULT false,
    max_size BIGINT NOT NULL DEFAULT 0,
    max_egress BIGINT NOT NULL DEFAULT 0,
    ttl BIGINT NOT NULL DEFAULT 0,
    created BIGINT NOT NULL DEFAULT 0,
    updated BIGINT NOT NULL DEFAULT 0,
    UNIQUE(org_id)
);

CREATE TABLE IF NOT EXISTS build_cache_usage (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL,
    org_id BIGINT NOT NULL,
    bucket_id BIGINT NOT NULL DEFAULT 0,
    day BIGINT NOT NULL,
    hits BIGINT NOT NULL DEFAULT 0,
    partial_hits BIGINT NOT NULL DEFAULT 0,
    misses BIGINT NOT NULL DEFAULT 0,
    denied BIGINT NOT NULL DEFAULT 0,
    saves BIGINT NOT NULL DEFAULT 0,
    egress BIGINT NOT NULL DEFAULT 0,
    ingress BIGINT NOT NULL DEFAULT 0,
    UNIQUE(repo_id, bucket_id, day)
);

CREATE TABLE IF NOT EXISTS repos (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER,
    forge_id INTEGER,
    forge_remote_id TEXT,
    org_id INTEGER,
    owner TEXT,
    name TEXT,
    full_name TEXT,
    avatar VARCHAR(500),
    forge_url VARCHAR(1000),
    clone VARCHAR(1000),
    clone_ssh VARCHAR(1000),
    branch VARCHAR(500),
    pr_enabled BOOLEAN DEFAULT true,
    timeout INTEGER,
    visibility VARCHAR(10),
    private BOOLEAN,
    trusted TEXT,
    require_approval VARCHAR(50),
    active BOOLEAN,
    allow_pr BOOLEAN,
    allow_deploy BOOLEAN,
    allow_manual BOOLEAN,
    deploy_team VARCHAR(250),
    config_path VARCHAR(500),
    hash VARCHAR(500),
    cancel_previous_pipeline_events TEXT,
    netrc_trusted TEXT,
    logs_keep_min INTEGER,
    logs_keep_duration VARCHAR(500),
    UNIQUE(forge_id, owner, name),
    UNIQUE(forge_id, full_name)
);

CREATE TABLE IF NOT EXISTS orgs (
    id BIGSERIAL PRIMARY KEY,
    forge_id INTEGER,
    name TEXT,
    is_user BOOLEAN,
    private BOOLEAN,
    UNIQUE(forge_id, name)
);

CREATE TABLE IF NOT EXISTS forges (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(250),
    url VARCHAR(500),
    client VARCHAR(250),
    client_secret VARCHAR(250),
    skip_verify BOOLEAN,
    oauth_host VARCHAR(250),
    icon VARCHAR(250),
    additional_options TEXT
);

CREATE TABLE IF NOT EXISTS pipelines (
    id BIGSERIAL PRIMARY KEY,
    repo_id INTEGER,
    org_id INTEGER,
    org_name TEXT,
    repo_full_name TEXT,
    number INTEGER,
    author TEXT,
    parent INTEGER,
    event TEXT,
    status TEXT,
    errors TEXT,
    created INTEGER NOT NULL DEFAULT 0,
    updated INTEGER NOT NULL DEFAULT 0,
    started INTEGER,
    finished INTEGER,
    deploy TEXT,
    deploy_task TEXT,
    "commit" TEXT,
    branch TEXT,
    ref TEXT,
    refspec TEXT,
    title TEXT,
    message TEXT,
    timestamp INTEGER,
    sender TEXT,
    avatar VARCHAR(500),
    email VARCHAR(500),
    forge_url TEXT,
    reviewer TEXT,
    reviewed INTEGER,
    changed_files TEXT,
    additional_variables TEXT,
    pr_labels TEXT,
    is_prerelease BOOLEAN,
    from_fork BOOLEAN,
    logs_deleted BOOLEAN,
    notify_override TEXT,
    UNIQUE(repo_id, number)
);

CREATE TABLE IF NOT EXISTS workflows (
    id BIGSERIAL PRIMARY KEY,
    pipeline_id INTEGER,
    pid INTEGER,
    name TEXT,
    pipeline_name TEXT,
    state TEXT,
    error TEXT,
    started INTEGER,
    finished INTEGER,
    agent_id INTEGER,
    agent_persistent_id VARCHAR(64),
    agent_name VARCHAR(255),
    platform TEXT,
    environ TEXT,
    depends_on TEXT,
    axis_id INTEGER,
    original_workflow_id INTEGER DEFAULT 0,
    attempt INTEGER DEFAULT 1,
    requires_approval BOOLEAN DEFAULT FALSE,
    approval_users TEXT,
    approval_teams TEXT,
    approval_state VARCHAR(16) NOT NULL DEFAULT '',
    approval_deadline BIGINT DEFAULT 0,
    approval_timeout_seconds BIGINT DEFAULT 0,
    UNIQUE(pipeline_id, pid)
);

CREATE TABLE IF NOT EXISTS workflow_approval_events (
    id BIGSERIAL PRIMARY KEY,
    workflow_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL DEFAULT 0,
    action TEXT NOT NULL,
    reason TEXT,
    created_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS steps (
    id BIGSERIAL PRIMARY KEY,
    uuid TEXT,
    pipeline_id INTEGER,
    pid INTEGER,
    ppid INTEGER,
    name TEXT,
    state TEXT,
    error TEXT,
    failure TEXT,
    exit_code INTEGER,
    started INTEGER,
    finished INTEGER,
    type TEXT,
    depends_on TEXT,
    original_step_id INTEGER DEFAULT 0,
    attempt INTEGER DEFAULT 1,
    UNIQUE(pipeline_id, pid)
);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    pid INTEGER,
    name TEXT,
    data BYTEA,
    labels TEXT,
    dependencies TEXT,
    run_on TEXT,
    dependencies_status TEXT,
    agent_id INTEGER,
    pipeline_id INTEGER,
    repo_id INTEGER,
    canceled INTEGER DEFAULT 0,
    canceled_at INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS log_entries (
    id BIGSERIAL PRIMARY KEY,
    step_id INTEGER,
    time INTEGER,
    line INTEGER,
    data BYTEA,
    created INTEGER,
    type INTEGER
);

CREATE TABLE IF NOT EXISTS proc_log_entries (
    id BIGSERIAL PRIMARY KEY,
    source_type TEXT NOT NULL,
    source_id BIGINT NOT NULL,
    time BIGINT NOT NULL,
    level TEXT NOT NULL,
    line TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS configs (
    id BIGSERIAL PRIMARY KEY,
    repo_id INTEGER,
    hash TEXT,
    name TEXT,
    data BYTEA,
    generated_from TEXT NOT NULL DEFAULT '',
    UNIQUE(repo_id, hash, name)
);

CREATE TABLE IF NOT EXISTS pipeline_configs (
    config_id INTEGER NOT NULL,
    pipeline_id INTEGER NOT NULL,
    UNIQUE(config_id, pipeline_id)
);

CREATE TABLE IF NOT EXISTS secrets (
    id BIGSERIAL PRIMARY KEY,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL,
    value TEXT,
    images TEXT,
    events TEXT,
    source VARCHAR(50) DEFAULT 'internal',
    created_at INTEGER,
    updated_at INTEGER,
    UNIQUE(org_id, repo_id, name)
);

CREATE TABLE IF NOT EXISTS secret_targets (
    id BIGSERIAL PRIMARY KEY,
    secret_id BIGINT NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER,
    UNIQUE(secret_id, org_id, repo_id)
);

CREATE TABLE IF NOT EXISTS registries (
    id BIGSERIAL PRIMARY KEY,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    address TEXT NOT NULL,
    username VARCHAR(2000),
    password TEXT,
    created_at INTEGER,
    updated_at INTEGER,
    UNIQUE(org_id, repo_id, address)
);

CREATE TABLE IF NOT EXISTS perms (
    user_id INTEGER NOT NULL,
    repo_id INTEGER NOT NULL,
    pull BOOLEAN,
    push BOOLEAN,
    admin BOOLEAN,
    synced INTEGER,
    created INTEGER,
    updated INTEGER,
    UNIQUE(user_id, repo_id)
);

CREATE TABLE IF NOT EXISTS agents (
    id BIGSERIAL PRIMARY KEY,
    created INTEGER,
    updated INTEGER,
    name TEXT,
    owner_id INTEGER,
    token TEXT,
    persistent_id VARCHAR(64),
    last_contact INTEGER,
    workflow_polling_healthy BOOLEAN,
    last_work INTEGER,
    platform VARCHAR(100),
    backend VARCHAR(100),
    hostname VARCHAR(255),
    capacity INTEGER,
    version TEXT,
    build_date TEXT,
    no_schedule BOOLEAN,
    priority INTEGER DEFAULT 0,
    custom_labels TEXT,
    limit_mem INTEGER,
    limit_cpu_quota INTEGER,
    host_cpus INTEGER,
    host_mem BIGINT,
    org_ids TEXT,
    protocol_version INTEGER,
    remote_update VARCHAR(20),
    restart_requested BOOLEAN,
    restart_queued BOOLEAN,
    auto_restart_version VARCHAR(255),
    provisioning TEXT
);

CREATE TABLE IF NOT EXISTS autoscalers (
    id BIGSERIAL PRIMARY KEY,
    created INTEGER,
    updated INTEGER,
    name TEXT UNIQUE NOT NULL,
    token TEXT,
    owner_id INTEGER,
    last_contact INTEGER,
    min_agents INTEGER,
    max_agents INTEGER,
    workflows_per_agent INTEGER,
    provider VARCHAR(50),
    instance_type VARCHAR(100),
    region VARCHAR(100),
    agent_labels TEXT,
    limit_cpu_quota VARCHAR(50),
    limit_mem VARCHAR(50),
    enabled BOOLEAN DEFAULT true,
    version TEXT,
    active_agents INTEGER,
    pending_agents INTEGER,
    pending_since INTEGER,
    pending_agent_ids TEXT,
    provision_timeout BIGINT,
    org_id INTEGER
);

CREATE TABLE IF NOT EXISTS crons (
    id BIGSERIAL PRIMARY KEY,
    name TEXT,
    repo_id INTEGER,
    creator_id INTEGER,
    next_exec INTEGER,
    schedule TEXT NOT NULL,
    created INTEGER NOT NULL DEFAULT 0,
    branch TEXT,
    fail_count INTEGER NOT NULL DEFAULT 0,
    fail_msg TEXT,
    disabled BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(name, repo_id)
);

CREATE TABLE IF NOT EXISTS redirections (
    id BIGSERIAL PRIMARY KEY,
    forge_id INTEGER,
    repo_id INTEGER,
    repo_full_name TEXT,
    UNIQUE(forge_id, repo_full_name)
);

CREATE TABLE IF NOT EXISTS server_configs (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS access_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL,
    token_hash VARCHAR(64) UNIQUE,
    scopes TEXT,
    expires_at INTEGER,
    last_used INTEGER,
    created_at INTEGER,
    updated_at INTEGER
);

CREATE TABLE IF NOT EXISTS integrations (
    id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    description TEXT,
    config TEXT,
    allowed_user_ids TEXT,
    allowed_org_ids TEXT,
    allowed_repo_ids TEXT,
    events TEXT,
    images TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at INTEGER,
    updated_at INTEGER
);

CREATE TABLE IF NOT EXISTS approved_pull_requests (
    id BIGSERIAL PRIMARY KEY,
    repo_id INTEGER,
    ref TEXT,
    approved_by TEXT,
    created INTEGER NOT NULL DEFAULT 0,
    UNIQUE(repo_id, ref)
);

CREATE TABLE IF NOT EXISTS distributed_locks (
    id BIGSERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    instance_id TEXT NOT NULL,
    acquired_at INTEGER,
    expires_at INTEGER
);

CREATE TABLE IF NOT EXISTS pubsub_messages (
    id TEXT PRIMARY KEY,
    data BYTEA,
    labels TEXT,
    created_at INTEGER,
    instance_id TEXT
);

CREATE TABLE IF NOT EXISTS cache_entries (
    key TEXT PRIMARY KEY,
    data BYTEA,
    expires_at INTEGER,
    created_at INTEGER
);

CREATE TABLE IF NOT EXISTS maintenance_config (
    id BIGSERIAL PRIMARY KEY,
    action_type TEXT UNIQUE,
    enabled BOOLEAN,
    schedule TEXT,
    dry_run BOOLEAN DEFAULT false,
    total_runs INTEGER DEFAULT 0,
    successful_runs INTEGER DEFAULT 0,
    last_run TIMESTAMP,
    last_run_success BOOLEAN DEFAULT false,
    last_run_duration INTEGER DEFAULT 0,
    last_run_error TEXT,
    created INTEGER,
    updated INTEGER,
    stale_threshold INTEGER DEFAULT 3600
);

CREATE TABLE IF NOT EXISTS maintenance_operation_config (
    id BIGSERIAL PRIMARY KEY,
    operation TEXT UNIQUE,
    enabled BOOLEAN,
    schedule TEXT,
    total_runs INTEGER,
    last_run TIMESTAMP,
    created INTEGER,
    updated INTEGER
);

CREATE TABLE IF NOT EXISTS maintenance_stats (
    id BIGSERIAL PRIMARY KEY,
    last_run TIMESTAMP,
    last_run_success BOOLEAN,
    last_run_error TEXT,
    last_run_duration INTEGER,
    total_runs INTEGER,
    successful_runs INTEGER,
    created INTEGER,
    updated INTEGER
);

CREATE TABLE IF NOT EXISTS maintenance_logs (
    id BIGSERIAL PRIMARY KEY,
    run_time TIMESTAMP,
    success BOOLEAN,
    error TEXT,
    duration INTEGER,
    details TEXT,
    created INTEGER
);

-- Indexes

CREATE INDEX IF NOT EXISTS idx_pipelines_repo_id ON pipelines (repo_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_org_id ON pipelines (org_id);
CREATE INDEX IF NOT EXISTS idx_pipelines_author ON pipelines (author);
CREATE INDEX IF NOT EXISTS idx_pipelines_status ON pipelines (status);

CREATE INDEX IF NOT EXISTS idx_steps_pipeline_id ON steps (pipeline_id);
CREATE INDEX IF NOT EXISTS idx_steps_uuid ON steps (uuid);

CREATE INDEX IF NOT EXISTS idx_log_entries_step_id ON log_entries (step_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_log_entries_step_id_line ON log_entries (step_id, line);

CREATE INDEX IF NOT EXISTS idx_proc_log_entries_source_id ON proc_log_entries (source_type, source_id, id);

CREATE INDEX IF NOT EXISTS idx_workflows_pipeline_id ON workflows (pipeline_id);
CREATE INDEX IF NOT EXISTS idx_workflows_approval ON workflows (approval_state, approval_deadline);

CREATE INDEX IF NOT EXISTS idx_workflow_approval_events_workflow_id ON workflow_approval_events (workflow_id);

CREATE INDEX IF NOT EXISTS idx_repos_user_id ON repos (user_id);
CREATE INDEX IF NOT EXISTS idx_repos_org_id ON repos (org_id);

CREATE INDEX IF NOT EXISTS idx_perms_user_id ON perms (user_id);
CREATE INDEX IF NOT EXISTS idx_perms_repo_id ON perms (repo_id);

CREATE INDEX IF NOT EXISTS idx_autoscalers_org_id ON autoscalers (org_id);

CREATE INDEX IF NOT EXISTS idx_crons_repo_id ON crons (repo_id);
CREATE INDEX IF NOT EXISTS idx_crons_creator_id ON crons (creator_id);

CREATE INDEX IF NOT EXISTS idx_user_forges_user_id ON user_forges (user_id);
CREATE INDEX IF NOT EXISTS idx_user_forges_forge_id ON user_forges (forge_id);
CREATE INDEX IF NOT EXISTS idx_user_identities_user_id ON user_identities (user_id);

CREATE INDEX IF NOT EXISTS idx_pipeline_watches_key ON pipeline_watches (repo_id, watch_event, watch_target);
CREATE INDEX IF NOT EXISTS idx_build_caches_last_used ON build_caches (last_used);
CREATE INDEX IF NOT EXISTS idx_build_cache_usage_org_day ON build_cache_usage (org_id, day);

CREATE INDEX IF NOT EXISTS idx_access_tokens_user_id ON access_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_access_tokens_org_id ON access_tokens (org_id);
CREATE INDEX IF NOT EXISTS idx_access_tokens_repo_id ON access_tokens (repo_id);
