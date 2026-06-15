CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_projects_user_created_at
    ON projects (user_id, created_at, id);

CREATE TABLE IF NOT EXISTS project_repositories (
    project_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    repository_owner TEXT NOT NULL,
    repository_name TEXT NOT NULL,
    repository_branch TEXT NOT NULL,
    connected_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_project_repositories_user_project
    ON project_repositories (user_id, project_id);

CREATE TABLE IF NOT EXISTS containers (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    image TEXT NOT NULL,
    url TEXT NOT NULL,
    ready BOOLEAN NOT NULL,
    reason TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    namespace TEXT NOT NULL,
    generation BIGINT NOT NULL,
    PRIMARY KEY (project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_containers_project_created_at
    ON containers (project_id, created_at, name);

ALTER TABLE containers ADD COLUMN IF NOT EXISTS custom_domain TEXT;
ALTER TABLE containers ADD COLUMN IF NOT EXISTS port INT NOT NULL DEFAULT 8080;
ALTER TABLE containers ADD COLUMN IF NOT EXISTS min_scale INT NOT NULL DEFAULT 0;
ALTER TABLE containers ADD COLUMN IF NOT EXISTS max_scale INT NOT NULL DEFAULT 20;
ALTER TABLE containers ADD COLUMN IF NOT EXISTS startup_script TEXT;

CREATE TABLE IF NOT EXISTS identity_users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    email TEXT,
    name TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS operations (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    error TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

ALTER TABLE operations ADD COLUMN IF NOT EXISTS resource_type TEXT;
ALTER TABLE operations ADD COLUMN IF NOT EXISTS resource_name TEXT;
ALTER TABLE operations ADD COLUMN IF NOT EXISTS user_id TEXT;
ALTER TABLE operations ADD COLUMN IF NOT EXISTS project_id TEXT;

CREATE INDEX IF NOT EXISTS idx_operations_status_resource_type
    ON operations (status, resource_type);

CREATE TABLE IF NOT EXISTS identity_sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES identity_users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_identity_sessions_user_expires
    ON identity_sessions (user_id, expires_at, token_hash);
