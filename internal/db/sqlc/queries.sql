-- name: ListProjects :many
SELECT id, user_id, name, created_at
FROM projects
WHERE user_id = $1
ORDER BY created_at, id;

-- name: ListProjectsWithDeleting :many
SELECT p.id, p.user_id, p.name, p.created_at,
       EXISTS (
           SELECT 1 FROM operations o
           WHERE o.project_id = p.id
             AND o.resource_type = 'project'
             AND o.status = 'pending'
       ) AS deleting
FROM projects p
WHERE p.user_id = $1
ORDER BY p.created_at, p.id;

-- name: IsProjectDeleting :one
SELECT EXISTS (
    SELECT 1 FROM operations
    WHERE project_id = $1
      AND resource_type = 'project'
      AND status = 'pending'
);

-- name: GetProjectRepository :one
SELECT project_id, user_id, repository_owner, repository_name, repository_branch, connected_at, updated_at
FROM project_repositories
WHERE user_id = $1 AND project_id = $2;

-- name: UpsertProjectRepository :one
INSERT INTO project_repositories (
    project_id, user_id, repository_owner, repository_name, repository_branch, connected_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (project_id) DO UPDATE SET
    user_id = EXCLUDED.user_id,
    repository_owner = EXCLUDED.repository_owner,
    repository_name = EXCLUDED.repository_name,
    repository_branch = EXCLUDED.repository_branch,
    updated_at = EXCLUDED.updated_at
RETURNING project_id, user_id, repository_owner, repository_name, repository_branch, connected_at, updated_at;

-- name: CreateProject :one
INSERT INTO projects (id, user_id, name, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, name) DO NOTHING
RETURNING id, user_id, name, created_at;

-- name: DeleteProject :execrows
DELETE FROM projects
WHERE user_id = $1 AND id = $2;

-- name: ProjectExists :one
SELECT EXISTS (
    SELECT 1
    FROM projects
    WHERE user_id = $1 AND id = $2
);

-- name: ListContainers :many
SELECT project_id, name, image, url, ready, reason, created_at, updated_at, namespace, generation, custom_domain, port, min_scale, max_scale, startup_script, env
FROM containers
WHERE project_id = $1
ORDER BY created_at, name;

-- name: GetContainer :one
SELECT project_id, name, image, url, ready, reason, created_at, updated_at, namespace, generation, custom_domain, port, min_scale, max_scale, startup_script, env
FROM containers
WHERE project_id = $1 AND name = $2;

-- name: UpsertContainer :one
INSERT INTO containers (
    project_id, name, image, url, ready, reason,
    created_at, updated_at, namespace, generation, port, min_scale, max_scale, startup_script, env
)
VALUES ($1, $2, $3, $4, TRUE, $5, $6, $7, $8, 1, $9, $10, $11, $12, $13)
ON CONFLICT (project_id, name) DO UPDATE SET
    image = EXCLUDED.image,
    url = EXCLUDED.url,
    ready = EXCLUDED.ready,
    reason = EXCLUDED.reason,
    updated_at = EXCLUDED.updated_at,
    namespace = EXCLUDED.namespace,
    generation = containers.generation + 1,
    port = EXCLUDED.port,
    min_scale = EXCLUDED.min_scale,
    max_scale = EXCLUDED.max_scale,
    startup_script = EXCLUDED.startup_script,
    env = EXCLUDED.env
RETURNING project_id, name, image, url, ready, reason, created_at, updated_at, namespace, generation, custom_domain, port, min_scale, max_scale, startup_script, env;

-- name: UpdateContainerDomain :execrows
UPDATE containers SET custom_domain = $3, updated_at = $4
WHERE project_id = $1 AND name = $2;

-- name: DeleteContainer :execrows
DELETE FROM containers
WHERE project_id = $1 AND name = $2;

-- name: CreateOperation :one
INSERT INTO operations (id, status, resource_type, resource_name, user_id, project_id, created_at, updated_at)
VALUES ($1, 'pending', $2, $3, $4, $5, $6, $6)
RETURNING id, status, error, resource_type, resource_name, user_id, project_id, created_at, updated_at;

-- name: ListPendingOperationsByResourceType :many
SELECT id, status, error, resource_type, resource_name, user_id, project_id, created_at, updated_at
FROM operations
WHERE status = 'pending' AND resource_type = $1;

-- name: UpdateOperation :exec
UPDATE operations
SET status = $2, error = $3, updated_at = $4
WHERE id = $1;

-- name: GetOperation :one
SELECT id, status, error, resource_type, resource_name, user_id, project_id, created_at, updated_at
FROM operations
WHERE id = $1;
