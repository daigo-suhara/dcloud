from __future__ import annotations

import os
import time
from typing import Any

from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.responses import RedirectResponse
from fastapi import Response
from app.routes.project import router as project_router
from app.identity_client import IdentityClient
from app.repository import Repository
from app.container_client import ContainerClient

app = FastAPI(title="DCloud API")

app.include_router(project_router, prefix="/project", tags=["project"])


def session_cookie_name() -> str:
    return os.getenv("DCLD_SESSION_COOKIE_NAME", "dcloud_session").strip() or "dcloud_session"


def session_cookie_secure() -> bool:
    value = os.getenv("DCLD_COOKIE_SECURE", "false").strip().lower()
    return value not in {"0", "false", "no", "off"}


def current_user(request: Request) -> dict[str, Any]:
    session_token = request.cookies.get(session_cookie_name(), "").strip()
    if not session_token:
        raise HTTPException(status_code=401, detail="ログインが必要です")
    try:
        return app.state.identity_client.me(session_token)
    except PermissionError as exc:
        raise HTTPException(status_code=401, detail="ログインが必要です") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


def set_session_cookie(response: Response, session_token: str) -> None:
    response.set_cookie(
        key=session_cookie_name(),
        value=session_token,
        httponly=True,
        secure=session_cookie_secure(),
        samesite="lax",
        path="/",
        max_age=60 * 60 * 24 * 30,
    )


def clear_session_cookie(response: Response) -> None:
    response.delete_cookie(key=session_cookie_name(), path="/")


@app.on_event("startup")
def startup() -> None:
    last_error: Exception | None = None
    for _ in range(60):
        try:
            app.state.identity_client = IdentityClient.new()
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
    if not hasattr(app.state, "repo") or not hasattr(app.state, "identity_client"):
        raise HTTPException(status_code=503, detail="starting")
    return {"status": "ready", "service": "api"}


@app.get("/api/v1/auth/me")
def auth_me(request: Request) -> dict[str, Any]:
    return current_user(request)


@app.get("/api/v1/auth/login")
def auth_login_page() -> RedirectResponse:
    return RedirectResponse(url="/login", status_code=302)


@app.post("/api/v1/auth/login")
def auth_login(body: dict[str, Any], response: Response) -> dict[str, Any]:
    email = str(body.get("email", body.get("username", ""))).strip()
    password = str(body.get("password", "")).strip()
    try:
        auth = app.state.identity_client.login(email, password)
    except PermissionError as exc:
        raise HTTPException(status_code=401, detail="メールアドレスまたはパスワードが違います") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc

    set_session_cookie(response, auth["sessionToken"])
    return {"user": auth["user"]}


@app.get("/api/v1/auth/register")
def auth_register_page() -> RedirectResponse:
    return RedirectResponse(url="/login", status_code=302)


@app.post("/api/v1/auth/register")
def auth_register(body: dict[str, Any], response: Response) -> dict[str, Any]:
    email = str(body.get("email", body.get("username", ""))).strip()
    password = str(body.get("password", "")).strip()
    try:
        auth = app.state.identity_client.register(email, password, "")
    except PermissionError as exc:
        raise HTTPException(status_code=401, detail="アカウントを作成できませんでした") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc

    set_session_cookie(response, auth["sessionToken"])
    return {"user": auth["user"]}


@app.get("/api/v1/auth/logout")
def auth_logout_page() -> RedirectResponse:
    return RedirectResponse(url="/login", status_code=302)


@app.post("/api/v1/auth/logout")
def auth_logout(request: Request, response: Response) -> dict[str, str]:
    session_token = request.cookies.get(session_cookie_name(), "").strip()
    if session_token:
        try:
            app.state.identity_client.logout(session_token)
        except RuntimeError:
            pass
    clear_session_cookie(response)
    return {"status": "ok"}


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
