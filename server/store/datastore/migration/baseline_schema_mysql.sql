-- Baseline schema for Crow CI (MySQL)
-- Generated from server/model/ bun struct tags

CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    forge_id INTEGER,
    forge_remote_id TEXT,
    primary_forge_id INTEGER,
    login VARCHAR(250) UNIQUE,
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
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
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
    UNIQUE(forge_id, forge_remote_id(255)),
    INDEX idx_user_forges_user_id (user_id),
    INDEX idx_user_forges_forge_id (forge_id)
);

CREATE TABLE IF NOT EXISTS user_identities (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id INTEGER NOT NULL,
    provider VARBINARY(50) NOT NULL,
    issuer VARBINARY(255) NOT NULL DEFAULT '',
    subject VARBINARY(255) NOT NULL,
    email VARCHAR(500),
    username VARCHAR(250),
    idp_groups TEXT,
    linked_at INTEGER NOT NULL,
    last_login INTEGER,
    UNIQUE(provider, issuer, subject),
    INDEX idx_user_identities_user_id (user_id)
);

CREATE TABLE IF NOT EXISTS build_caches (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    repo_id BIGINT NOT NULL,
    scope VARBINARY(255) NOT NULL,
    cache_key VARBINARY(255) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    committed BOOLEAN NOT NULL DEFAULT false,
    created BIGINT NOT NULL DEFAULT 0,
    last_used BIGINT NOT NULL DEFAULT 0,
    UNIQUE(repo_id, scope, cache_key),
    INDEX idx_build_caches_last_used (last_used)
);

CREATE TABLE IF NOT EXISTS repos (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id INTEGER,
    forge_id INTEGER,
    forge_remote_id TEXT,
    org_id INTEGER,
    owner VARCHAR(250),
    name VARCHAR(250),
    full_name VARCHAR(500),
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
    UNIQUE(forge_id, full_name),
    INDEX idx_repos_user_id (user_id),
    INDEX idx_repos_org_id (org_id)
);

CREATE TABLE IF NOT EXISTS orgs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    forge_id INTEGER,
    name VARCHAR(250),
    is_user BOOLEAN,
    private BOOLEAN,
    UNIQUE(forge_id, name)
);

CREATE TABLE IF NOT EXISTS forges (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
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
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
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
    `commit` TEXT,
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
    UNIQUE(repo_id, number),
    INDEX idx_pipelines_repo_id (repo_id),
    INDEX idx_pipelines_org_id (org_id),
    INDEX idx_pipelines_author (author(255)),
    INDEX idx_pipelines_status (status(255))
);

CREATE TABLE IF NOT EXISTS workflows (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
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
    UNIQUE(pipeline_id, pid),
    INDEX idx_workflows_pipeline_id (pipeline_id),
    INDEX idx_workflows_approval (approval_state, approval_deadline)
);

CREATE TABLE IF NOT EXISTS workflow_approval_events (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    workflow_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL DEFAULT 0,
    action TEXT NOT NULL,
    reason TEXT,
    created_at BIGINT NOT NULL,
    INDEX idx_workflow_approval_events_workflow_id (workflow_id)
);

CREATE TABLE IF NOT EXISTS steps (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
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
    UNIQUE(pipeline_id, pid),
    INDEX idx_steps_pipeline_id (pipeline_id),
    INDEX idx_steps_uuid (uuid(255))
);

CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(255) PRIMARY KEY,
    pid INTEGER,
    name TEXT,
    data BLOB,
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
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    step_id INTEGER,
    time INTEGER,
    line INTEGER,
    data BLOB,
    created INTEGER,
    type INTEGER,
    INDEX idx_log_entries_step_id (step_id),
    UNIQUE KEY idx_log_entries_step_id_line (step_id, line)
);

CREATE TABLE IF NOT EXISTS proc_log_entries (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    source_type TEXT NOT NULL,
    source_id BIGINT NOT NULL,
    time BIGINT NOT NULL,
    level TEXT NOT NULL,
    line TEXT NOT NULL,
    INDEX idx_proc_log_entries_source_id (source_type(32), source_id, id)
);

CREATE TABLE IF NOT EXISTS configs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    repo_id INTEGER,
    hash VARCHAR(250),
    name VARCHAR(250),
    data BLOB,
    generated_from VARCHAR(250) NOT NULL DEFAULT '',
    UNIQUE(repo_id, hash, name)
);

CREATE TABLE IF NOT EXISTS pipeline_configs (
    config_id INTEGER NOT NULL,
    pipeline_id INTEGER NOT NULL,
    UNIQUE(config_id, pipeline_id)
);

CREATE TABLE IF NOT EXISTS secrets (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(250) NOT NULL,
    value TEXT,
    images TEXT,
    events TEXT,
    source VARCHAR(50) DEFAULT 'internal',
    created_at INTEGER,
    updated_at INTEGER,
    UNIQUE(org_id, repo_id, name)
);

CREATE TABLE IF NOT EXISTS secret_targets (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    secret_id BIGINT NOT NULL,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER,
    UNIQUE(secret_id, org_id, repo_id),
    FOREIGN KEY (secret_id) REFERENCES secrets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS registries (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    address VARCHAR(250) NOT NULL,
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
    UNIQUE(user_id, repo_id),
    INDEX idx_perms_user_id (user_id),
    INDEX idx_perms_repo_id (repo_id)
);

CREATE TABLE IF NOT EXISTS agents (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
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
    org_ids TEXT
);

CREATE TABLE IF NOT EXISTS autoscalers (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    created INTEGER,
    updated INTEGER,
    name VARCHAR(250) UNIQUE NOT NULL,
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
    org_id INTEGER,
    INDEX idx_autoscalers_org_id (org_id)
);

CREATE TABLE IF NOT EXISTS crons (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(250),
    repo_id INTEGER,
    creator_id INTEGER,
    next_exec INTEGER,
    schedule TEXT NOT NULL,
    created INTEGER NOT NULL DEFAULT 0,
    branch TEXT,
    fail_count INTEGER NOT NULL DEFAULT 0,
    fail_msg TEXT,
    disabled BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(name, repo_id),
    INDEX idx_crons_repo_id (repo_id),
    INDEX idx_crons_creator_id (creator_id)
);

CREATE TABLE IF NOT EXISTS redirections (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    forge_id INTEGER,
    repo_id INTEGER,
    repo_full_name TEXT,
    UNIQUE(forge_id, repo_full_name(255))
);

CREATE TABLE IF NOT EXISTS server_configs (
    `key` VARCHAR(250) PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS access_tokens (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id INTEGER NOT NULL,
    org_id INTEGER NOT NULL DEFAULT 0,
    repo_id INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(250) NOT NULL,
    token_hash VARCHAR(64) UNIQUE,
    scopes TEXT,
    expires_at INTEGER,
    last_used INTEGER,
    created_at INTEGER,
    updated_at INTEGER,
    INDEX idx_access_tokens_user_id (user_id),
    INDEX idx_access_tokens_org_id (org_id),
    INDEX idx_access_tokens_repo_id (repo_id)
);

CREATE TABLE IF NOT EXISTS integrations (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
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

CREATE TABLE IF NOT EXISTS notification_configs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    repo_id INTEGER NOT NULL UNIQUE,
    is_enabled BOOLEAN DEFAULT true,
    events TEXT,
    created_at INTEGER,
    updated_at INTEGER
);

CREATE TABLE IF NOT EXISTS email_unsubscribes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id INTEGER NOT NULL,
    repo_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    created_at INTEGER,
    UNIQUE(user_id, repo_id)
);

CREATE TABLE IF NOT EXISTS approved_pull_requests (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    repo_id INTEGER,
    ref TEXT,
    approved_by TEXT,
    created INTEGER NOT NULL DEFAULT 0,
    UNIQUE(repo_id, ref(255))
);

CREATE TABLE IF NOT EXISTS distributed_locks (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(250) UNIQUE NOT NULL,
    instance_id VARCHAR(250) NOT NULL,
    acquired_at INTEGER,
    expires_at INTEGER
);

CREATE TABLE IF NOT EXISTS pubsub_messages (
    id VARCHAR(255) PRIMARY KEY,
    data BLOB,
    labels TEXT,
    created_at INTEGER,
    instance_id VARCHAR(250)
);

CREATE TABLE IF NOT EXISTS cache_entries (
    `key` VARCHAR(250) PRIMARY KEY,
    data BLOB,
    expires_at INTEGER,
    created_at INTEGER
);

CREATE TABLE IF NOT EXISTS maintenance_config (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    action_type VARCHAR(250) UNIQUE,
    enabled BOOLEAN,
    schedule TEXT,
    dry_run BOOLEAN DEFAULT false,
    total_runs INTEGER DEFAULT 0,
    successful_runs INTEGER DEFAULT 0,
    last_run DATETIME,
    last_run_success BOOLEAN DEFAULT false,
    last_run_duration INTEGER DEFAULT 0,
    last_run_error TEXT,
    created INTEGER,
    updated INTEGER,
    stale_threshold INTEGER DEFAULT 3600
);

CREATE TABLE IF NOT EXISTS maintenance_operation_config (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    operation VARCHAR(250) UNIQUE,
    enabled BOOLEAN,
    schedule TEXT,
    total_runs INTEGER,
    last_run DATETIME,
    created INTEGER,
    updated INTEGER
);

CREATE TABLE IF NOT EXISTS maintenance_stats (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    last_run DATETIME,
    last_run_success BOOLEAN,
    last_run_error TEXT,
    last_run_duration INTEGER,
    total_runs INTEGER,
    successful_runs INTEGER,
    created INTEGER,
    updated INTEGER
);

CREATE TABLE IF NOT EXISTS maintenance_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_time DATETIME,
    success BOOLEAN,
    error TEXT,
    duration INTEGER,
    details TEXT,
    created INTEGER
);

-- Indexes are declared inline within each CREATE TABLE above.
-- MySQL has no portable "CREATE INDEX IF NOT EXISTS", so keeping index
-- definitions inside the (idempotent) CREATE TABLE IF NOT EXISTS statements
-- lets the baseline be re-run safely without "Duplicate key name" errors.
