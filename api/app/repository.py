from __future__ import annotations

import os
from dataclasses import dataclass
from datetime import datetime, timezone
from threading import Lock
from typing import Any
from uuid import uuid4

import psycopg
from psycopg.rows import dict_row


def now() -> str:
    return datetime.now(tz=timezone.utc).isoformat().replace("+00:00", "Z")


def env(key: str, fallback: str) -> str:
    value = os.getenv(key)
    return value if value else fallback


def database_url() -> str:
    return env(
        "DCLD_DATABASE_URL",
        "postgresql://postgres:postgres@127.0.0.1:5432/dcloud?sslmode=disable",
    )


def sanitize_dns_label(value: str) -> str:
    value = value.strip().lower()
    chars: list[str] = []
    last_hyphen = False
    for ch in value:
        if ("a" <= ch <= "z") or ("0" <= ch <= "9"):
            chars.append(ch)
            last_hyphen = False
        elif chars and not last_hyphen:
            chars.append("-")
            last_hyphen = True
    return "".join(chars).strip("-")


def short_id() -> str:
    return uuid4().hex[:8]


@dataclass
class Repository:
    lock: Lock
    dsn: str

    @classmethod
    def new(cls) -> "Repository":
        repo = cls(lock=Lock(), dsn=database_url())
        repo.ping()
        return repo

    def _connect(self) -> psycopg.Connection[Any]:
        return psycopg.connect(self.dsn, row_factory=dict_row)

    def ping(self) -> None:
        with self._connect() as conn, conn.cursor() as cur:
            cur.execute("SELECT to_regclass('public.identity_sessions') IS NOT NULL AS ready")
            row = cur.fetchone()
            if row is None or not row["ready"]:
                raise RuntimeError("database schema is not ready")

    def _project_exists(self, conn: psycopg.Connection[Any], user_id: str, project_id: str) -> bool:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT 1 FROM projects WHERE user_id = %s AND id = %s LIMIT 1",
                (user_id, project_id),
            )
            return cur.fetchone() is not None

    def project_exists(self, user_id: str, project_id: str) -> bool:
        normalized_user = user_id.strip()
        normalized_project = project_id.strip()
        if not normalized_user or not normalized_project:
            return False
        with self._connect() as conn:
            return self._project_exists(conn, normalized_user, normalized_project)

    def get_repository(self, user_id: str, project_id: str) -> dict[str, Any] | None:
        normalized_user = user_id.strip()
        normalized_project = project_id.strip()
        if not normalized_user or not normalized_project:
            raise ValueError("userId and projectId are required")
        with self._connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                SELECT project_id AS "projectId",
                       user_id AS "userId",
                       repository_owner AS "repositoryOwner",
                       repository_name AS "repositoryName",
                       repository_branch AS "repositoryBranch",
                       connected_at AS "connectedAt",
                       updated_at AS "updatedAt"
                FROM project_repositories
                WHERE user_id = %s AND project_id = %s
                """,
                (normalized_user, normalized_project),
            )
            row = cur.fetchone()
            return dict(row) if row is not None else None

    def upsert_repository(
        self,
        user_id: str,
        project_id: str,
        repository_owner: str,
        repository_name: str,
        repository_branch: str,
    ) -> dict[str, Any]:
        normalized_user = user_id.strip()
        normalized_project = project_id.strip()
        normalized_owner = repository_owner.strip()
        normalized_name = repository_name.strip()
        normalized_branch = repository_branch.strip() or "main"
        if not normalized_user or not normalized_project:
            raise ValueError("userId and projectId are required")
        if not normalized_owner or not normalized_name:
            raise ValueError("repositoryOwner and repositoryName are required")
        if not self.project_exists(normalized_user, normalized_project):
            raise KeyError("project not found")

        timestamp = now()
        with self.lock, self._connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO project_repositories (
                    project_id,
                    user_id,
                    repository_owner,
                    repository_name,
                    repository_branch,
                    connected_at,
                    updated_at
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s)
                ON CONFLICT (project_id) DO UPDATE SET
                    user_id = EXCLUDED.user_id,
                    repository_owner = EXCLUDED.repository_owner,
                    repository_name = EXCLUDED.repository_name,
                    repository_branch = EXCLUDED.repository_branch,
                    updated_at = EXCLUDED.updated_at
                RETURNING
                    project_id AS "projectId",
                    user_id AS "userId",
                    repository_owner AS "repositoryOwner",
                    repository_name AS "repositoryName",
                    repository_branch AS "repositoryBranch",
                    connected_at AS "connectedAt",
                    updated_at AS "updatedAt"
                """,
                (
                    normalized_project,
                    normalized_user,
                    normalized_owner,
                    normalized_name,
                    normalized_branch,
                    timestamp,
                    timestamp,
                ),
            )
            row = cur.fetchone()
            assert row is not None
            return dict(row)

    def list_projects(self, user_id: str) -> list[dict[str, Any]]:
        normalized_user = user_id.strip()
        if not normalized_user:
            raise ValueError("userId is required")
        with self._connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                SELECT id, name, user_id AS owner, created_at AS "createdAt"
                FROM projects
                WHERE user_id = %s
                ORDER BY created_at, id
                """,
                (normalized_user,),
            )
            return [dict(row) for row in cur.fetchall()]

    def create_project(self, user_id: str, name: str) -> dict[str, Any]:
        normalized_user = user_id.strip()
        normalized_name = name.strip()
        if not normalized_user or not normalized_name:
            raise ValueError("userId and name are required")

        project = {
            "id": f"{sanitize_dns_label(normalized_name)}-{short_id()}",
            "name": normalized_name,
            "owner": normalized_user,
            "createdAt": now(),
        }

        with self.lock, self._connect() as conn, conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO projects (id, user_id, name, created_at)
                VALUES (%s, %s, %s, %s)
                ON CONFLICT (user_id, name) DO NOTHING
                RETURNING id, name, user_id AS owner, created_at AS "createdAt"
                """,
                (project["id"], normalized_user, normalized_name, project["createdAt"]),
            )
            row = cur.fetchone()
            if row is None:
                raise KeyError("project already exists")
            return dict(row)

    def delete_project(self, user_id: str, project_id: str) -> bool:
        normalized_user = user_id.strip()
        normalized_project = project_id.strip()
        if not normalized_user or not normalized_project:
            raise ValueError("userId and projectId are required")

        with self.lock, self._connect() as conn, conn.cursor() as cur:
            cur.execute(
                "DELETE FROM projects WHERE user_id = %s AND id = %s",
                (normalized_user, normalized_project),
            )
            return cur.rowcount > 0
