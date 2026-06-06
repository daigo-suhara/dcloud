from __future__ import annotations

import os
import time
from typing import Any

from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.responses import RedirectResponse
from app.routes.container import router as container_router
from app.routes.project import router as project_router
from app.repository import Repository
from app.container_client import ContainerClient

app = FastAPI(title="DCloud API")

app.include_router(project_router, prefix="/project", tags=["project"])
app.include_router(container_router, prefix="/container", tags=["container"])


def current_user(request: Request) -> dict[str, Any]:
    uid = request.headers.get("X-authentik-uid", "").strip()
    username = request.headers.get("X-authentik-username", "").strip()
    if not uid or not username:
        raise HTTPException(status_code=401, detail="ログインが必要です")
    return {
        "id": uid,
        "username": username,
        "email": request.headers.get("X-authentik-email", "").strip() or None,
        "name": request.headers.get("X-authentik-name", "").strip() or None,
    }


def authentik_login_url() -> str:
    return os.getenv("DCLD_AUTHENTIK_LOGIN_URL", "/outpost.goauthentik.io/start?rd=/").strip() or "/outpost.goauthentik.io/start?rd=/"


def authentik_logout_url() -> str:
    return os.getenv("DCLD_AUTHENTIK_LOGOUT_URL", "/outpost.goauthentik.io/sign_out").strip() or "/outpost.goauthentik.io/sign_out"


@app.on_event("startup")
def startup() -> None:
    last_error: Exception | None = None
    for _ in range(60):
        try:
            app.state.repo = Repository.new()
            app.state.container_client = ContainerClient.new()
            return
        except Exception as exc:  # pragma: no cover - startup retry path
            last_error = exc
            time.sleep(1)
    if last_error is not None:
        raise last_error


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok", "service": "api"}


@app.get("/readyz")
def readyz() -> dict[str, str]:
    if not hasattr(app.state, "repo"):
        raise HTTPException(status_code=503, detail="starting")
    return {"status": "ready", "service": "api"}


@app.get("/api/v1/auth/me")
def auth_me(request: Request) -> dict[str, Any]:
    return current_user(request)


@app.get("/api/v1/auth/login")
@app.post("/api/v1/auth/login")
def auth_login() -> RedirectResponse:
    return RedirectResponse(url=authentik_login_url(), status_code=302)


@app.get("/api/v1/auth/logout")
@app.post("/api/v1/auth/logout")
def auth_logout() -> RedirectResponse:
    return RedirectResponse(url=authentik_logout_url(), status_code=302)


@app.get("/api/v1/projects")
def list_projects(request: Request) -> dict[str, Any]:
    user = current_user(request)
    return {"user": user["id"], "projects": app.state.repo.list_projects(user["id"])}


@app.post("/api/v1/projects")
def create_project(body: dict[str, Any], request: Request) -> dict[str, Any]:
    user = current_user(request)
    name = str(body.get("name", "")).strip()
    if not name:
        raise HTTPException(status_code=400, detail="プロジェクト名は必須です")

    try:
        return app.state.repo.create_project(user["id"], name)
    except KeyError as exc:
        raise HTTPException(status_code=409, detail="プロジェクトは既に存在します") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc


@app.delete("/api/v1/projects/{project_id}")
def delete_project(project_id: str, request: Request) -> dict[str, str]:
    user = current_user(request)
    try:
        deleted = app.state.repo.delete_project(user["id"], project_id)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    if not deleted:
        raise HTTPException(status_code=404, detail="プロジェクトが見つかりません")
    return {"status": "deleted"}


@app.get("/api/v1/projects/{project_id}/repository")
def get_project_repository(project_id: str, request: Request) -> dict[str, Any]:
    user = current_user(request)
    try:
        repository = app.state.repo.get_repository(user["id"], project_id)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    if repository is None:
        raise HTTPException(status_code=404, detail="リポジトリ設定が見つかりません")
    return repository


@app.put("/api/v1/projects/{project_id}/repository")
def upsert_project_repository(project_id: str, body: dict[str, Any], request: Request) -> dict[str, Any]:
    user = current_user(request)
    repository_owner = str(body.get("repositoryOwner", "")).strip()
    repository_name = str(body.get("repositoryName", "")).strip()
    repository_branch = str(body.get("repositoryBranch", "main")).strip() or "main"
    try:
        return app.state.repo.upsert_repository(
            user["id"],
            project_id,
            repository_owner,
            repository_name,
            repository_branch,
        )
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="プロジェクトが見つかりません") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc


@app.get("/api/v1/container")
def list_container(
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        containers = app.state.container_client.list_services(user["id"], project_id)
        return {"namespace": containers["namespace"], "user": user["id"], "projectId": project_id, "containers": containers["containers"]}
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービス一覧を取得できません") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.post("/api/v1/container")
def deploy_container(
    body: dict[str, Any],
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    name = str(body.get("name", "")).strip()
    image = str(body.get("image", "")).strip()
    try:
        return app.state.container_client.deploy_service(
            user["id"],
            project_id,
            name,
            image,
            int(body.get("port", 8080) or 8080),
            int(body.get("minScale", 1) or 1),
            int(body.get("maxScale", 1) or 1),
        )
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービスを作成できません") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.delete("/api/v1/container/{name}")
def delete_container(
    name: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, str]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        app.state.container_client.delete_service(user["id"], project_id, name)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービスが見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    return {"status": "deleted"}
