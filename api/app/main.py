from __future__ import annotations

import asyncio
import ssl
import os
import time
from hashlib import sha256
from typing import Any
from urllib.parse import quote

import boto3
from botocore.config import Config as BotocoreConfig

from fastapi import FastAPI, File, Header, HTTPException, Request, UploadFile
from fastapi import WebSocket, WebSocketDisconnect
from fastapi.responses import RedirectResponse, StreamingResponse
from fastapi import Response
from app.routes.project import router as project_router
from app.identity_client import IdentityClient
from app.compute_client import ComputeClient
from app.repository import Repository
from app.container_client import ContainerClient
from app.storage_client import StorageClient
from app.database_client import DatabaseClient

app = FastAPI(title="DCloud API")

app.include_router(project_router, prefix="/project", tags=["project"])


def session_cookie_name() -> str:
    return os.getenv("DCLD_SESSION_COOKIE_NAME", "dcloud_session").strip() or "dcloud_session"


def session_cookie_secure() -> bool:
    value = os.getenv("DCLD_COOKIE_SECURE", "false").strip().lower()
    return value not in {"0", "false", "no", "off"}


def current_user(request: Request) -> dict[str, Any]:
    return current_user_from_session(request.cookies.get(session_cookie_name(), "").strip())


def current_user_from_session(session_token: str) -> dict[str, Any]:
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


def exception_detail(exc: Exception, fallback: str) -> str:
    message = str(exc).strip()
    if message.startswith("'") and message.endswith("'") and len(message) >= 2:
        message = message[1:-1].strip()
    return message or fallback


def ensure_project_not_deleting(project_id: str) -> None:
    if app.state.repo.is_project_deleting(project_id):
        raise HTTPException(status_code=409, detail="プロジェクトは削除中のためリソース操作は受け付けられません")


def compute_machine_resource_name(user_id: str, project_id: str, name: str) -> str:
    digest = sha256(f"{user_id.strip()}:{project_id.strip()}:{name.strip()}".encode("utf-8")).hexdigest()
    return f"vm-{digest[:16]}"


def resolve_compute_machine(user_id: str, project_id: str, name: str) -> dict[str, Any]:
    machines = app.state.compute_client.list_machines(user_id, project_id)
    for machine in machines["machines"]:
        if machine["name"] == name:
            return {"namespace": machines["namespace"], "machine": machine}
    raise HTTPException(status_code=404, detail="仮想マシンが見つかりません")


@app.on_event("startup")
def startup() -> None:
    last_error: Exception | None = None
    for _ in range(60):
        try:
            app.state.identity_client = IdentityClient.new()
            app.state.repo = Repository.new()
            app.state.container_client = ContainerClient.new()
            app.state.compute_client = ComputeClient.new()
            app.state.storage_client = StorageClient.new()
            app.state.database_client = DatabaseClient.new()
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
    if not hasattr(app.state, "repo") or not hasattr(app.state, "identity_client") or not hasattr(app.state, "compute_client"):
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
    import secrets
    user = current_user(request)
    if not app.state.repo.project_exists(user["id"], project_id):
        raise HTTPException(status_code=404, detail="プロジェクトが見つかりません")
    ensure_project_not_deleting(project_id)
    try:
        machines = app.state.compute_client.list_machines(user["id"], project_id)
        for machine in machines.get("machines", []):
            try:
                app.state.compute_client.delete_machine(user["id"], project_id, machine["name"])
            except Exception:
                pass
    except Exception:
        pass
    try:
        services = app.state.container_client.list_services(user["id"], project_id)
        for container in services.get("containers", []):
            try:
                app.state.container_client.delete_service(user["id"], project_id, container["name"])
            except Exception:
                pass
    except Exception:
        pass
    op_id = "project-op-" + secrets.token_hex(8)
    try:
        app.state.repo.create_operation(op_id, "project", project_id, user["id"], project_id)
    except Exception:
        pass
    return {"status": "deleting", "operationId": op_id}


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
    ensure_project_not_deleting(project_id)
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
    ensure_project_not_deleting(project_id)
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
            str(body.get("startupScript", "") or ""),
            body.get("env") if isinstance(body.get("env"), list) else None,
        )
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービスを作成できません") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.put("/api/v1/container/{name}")
def update_container(
    name: str,
    body: dict[str, Any],
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    ensure_project_not_deleting(project_id)
    image = str(body.get("image", "")).strip()
    try:
        return app.state.container_client.deploy_service(
            user["id"],
            project_id,
            name,
            image,
            int(body.get("port", 8080) or 8080),
            int(body.get("minScale", 0) or 0),
            int(body.get("maxScale", 20) or 20),
            str(body.get("startupScript", "") or ""),
            body.get("env") if isinstance(body.get("env"), list) else None,
        )
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービスが見つかりません") from exc
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
        operation_id = app.state.container_client.delete_service(user["id"], project_id, name)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービスが見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    return {"status": "deleting", "operationId": operation_id}


@app.put("/api/v1/container/{name}/domain")
def set_container_domain(
    name: str,
    body: dict[str, Any],
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    ensure_project_not_deleting(project_id)
    custom_domain = str(body.get("customDomain", "")).strip()
    try:
        return app.state.container_client.set_service_domain(user["id"], project_id, name, custom_domain)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービスが見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.get("/api/v1/operations/{operation_id}")
def get_operation(operation_id: str, request: Request) -> dict[str, str]:
    current_user(request)
    if operation_id.startswith("container-op-"):
        client = app.state.container_client
    elif operation_id.startswith("compute-op-") or operation_id.startswith("project-op-"):
        client = app.state.compute_client
    elif operation_id.startswith("storage-op-"):
        client = app.state.storage_client
    elif operation_id.startswith("database-op-"):
        client = app.state.database_client
    else:
        raise HTTPException(status_code=404, detail="オペレーションが見つかりません")
    try:
        return client.get_operation(operation_id)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="オペレーションが見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.get("/api/v1/compute")
def list_compute(
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        machines = app.state.compute_client.list_machines(user["id"], project_id)
        return {"namespace": machines["namespace"], "user": user["id"], "projectId": project_id, "machines": machines["machines"]}
    except KeyError as exc:
        raise HTTPException(status_code=404, detail=exception_detail(exc, "仮想マシン一覧を取得できません")) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=exception_detail(exc, "仮想マシン一覧を取得できません")) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=exception_detail(exc, "仮想マシン一覧を取得できません")) from exc


@app.get("/api/v1/compute/{name}")
def get_compute(
    name: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        resolved = resolve_compute_machine(user["id"], project_id, name)
        return {"namespace": resolved["namespace"], "user": user["id"], "projectId": project_id, "machine": resolved["machine"]}
    except HTTPException:
        raise
    except KeyError as exc:
        raise HTTPException(status_code=404, detail=exception_detail(exc, "仮想マシンを取得できません")) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=exception_detail(exc, "仮想マシンを取得できません")) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=exception_detail(exc, "仮想マシンを取得できません")) from exc


@app.post("/api/v1/compute")
def create_compute(
    body: dict[str, Any],
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    ensure_project_not_deleting(project_id)
    name = str(body.get("name", "")).strip()
    image = str(body.get("image", "")).strip()
    cpu = str(body.get("cpu", "1")).strip() or "1"
    memory = str(body.get("memory", "1Gi")).strip() or "1Gi"
    try:
        return app.state.compute_client.create_machine(user["id"], project_id, name, image, cpu, memory)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail=exception_detail(exc, "仮想マシンを作成できません")) from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=exception_detail(exc, "仮想マシンを作成できません")) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=exception_detail(exc, "仮想マシンを作成できません")) from exc


@app.delete("/api/v1/compute/{name}")
def delete_compute(
    name: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, str]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        operation_id = app.state.compute_client.delete_machine(user["id"], project_id, name)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=exception_detail(exc, "仮想マシンを削除できません")) from exc
    except KeyError as exc:
        raise HTTPException(status_code=404, detail=exception_detail(exc, "仮想マシンが見つかりません")) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=exception_detail(exc, "仮想マシンを削除できません")) from exc
    return {"status": "deleting", "operationId": operation_id}


@app.get("/api/v1/storage")
def list_storage(
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        result = app.state.storage_client.list_buckets(user["id"], project_id)
        return {"user": user["id"], "projectId": project_id, "buckets": result["buckets"]}
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.post("/api/v1/storage")
def create_bucket(
    body: dict[str, Any],
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    ensure_project_not_deleting(project_id)
    name = str(body.get("name", "")).strip()
    if not name:
        raise HTTPException(status_code=400, detail="バケット名は必須です")
    try:
        return app.state.storage_client.create_bucket(user["id"], project_id, name)
    except KeyError as exc:
        raise HTTPException(status_code=409, detail="バケットは既に存在します") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.delete("/api/v1/storage/{name}")
def delete_bucket(
    name: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, str]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        operation_id = app.state.storage_client.delete_bucket(user["id"], project_id, name)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="バケットが見つかりません") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    return {"status": "deleting", "operationId": operation_id}


@app.get("/api/v1/storage/{name}/credentials")
def get_bucket_credentials(
    name: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        return app.state.storage_client.get_bucket_credentials(user["id"], project_id, name)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="バケットまたは認証情報が見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.get("/api/v1/database")
def list_databases(
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        result = app.state.database_client.list_databases(user["id"], project_id)
        return {"user": user["id"], "projectId": project_id, "databases": result["databases"]}
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.post("/api/v1/database")
def create_database(
    body: dict[str, Any],
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    ensure_project_not_deleting(project_id)
    name = str(body.get("name", "")).strip()
    db_type = str(body.get("type", "")).strip()
    if not name or not db_type:
        raise HTTPException(status_code=400, detail="名前とタイプは必須です")
    try:
        return app.state.database_client.create_database(
            user["id"],
            project_id,
            name,
            db_type,
            str(body.get("version", "")).strip(),
            str(body.get("cpu", "")).strip(),
            str(body.get("memory", "")).strip(),
            str(body.get("storage", "")).strip(),
        )
    except KeyError as exc:
        raise HTTPException(status_code=409, detail="データベースは既に存在します") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


@app.delete("/api/v1/database/{name}")
def delete_database(
    name: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, str]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        operation_id = app.state.database_client.delete_database(user["id"], project_id, name)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="データベースが見つかりません") from exc
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    return {"status": "deleting", "operationId": operation_id}


@app.get("/api/v1/database/{name}/connection")
def get_database_connection(
    name: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    try:
        return app.state.database_client.get_connection_string(user["id"], project_id, name)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="データベースが見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc


def _s3_client(user_id: str, project_id: str, bucket_name: str) -> tuple[Any, str]:
    try:
        creds = app.state.storage_client.get_bucket_credentials(user_id, project_id, bucket_name)
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="バケットが見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    client = boto3.client(
        "s3",
        endpoint_url=creds["endpoint"],
        aws_access_key_id=creds["accessKeyId"],
        aws_secret_access_key=creds["secretAccessKey"],
        config=BotocoreConfig(signature_version="s3v4", s3={"addressing_style": "path"}),
    )
    return client, creds["bucketName"]


@app.get("/api/v1/storage/{name}/objects")
def list_bucket_objects(
    name: str,
    request: Request,
    prefix: str = "",
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    client, bucket_name = _s3_client(user["id"], project_id, name)
    try:
        resp = client.list_objects_v2(Bucket=bucket_name, Prefix=prefix, Delimiter="/")
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    objects = [
        {"key": obj["Key"], "size": obj["Size"], "lastModified": obj["LastModified"].isoformat()}
        for obj in resp.get("Contents", [])
        if obj["Key"] != prefix
    ]
    prefixes = [p["Prefix"] for p in resp.get("CommonPrefixes", [])]
    return {"objects": objects, "prefixes": prefixes, "prefix": prefix}


@app.post("/api/v1/storage/{name}/objects")
async def upload_bucket_object(
    name: str,
    request: Request,
    file: UploadFile = File(...),
    prefix: str = "",
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, Any]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    client, bucket_name = _s3_client(user["id"], project_id, name)
    key = prefix + (file.filename or "upload")
    try:
        client.upload_fileobj(
            file.file,
            bucket_name,
            key,
            ExtraArgs={"ContentType": file.content_type or "application/octet-stream"},
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    return {"key": key}


@app.delete("/api/v1/storage/{name}/objects")
def delete_bucket_object(
    name: str,
    key: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
) -> dict[str, str]:
    user = current_user(request)
    project_id = (x_dcp_project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    client, bucket_name = _s3_client(user["id"], project_id, name)
    try:
        client.delete_object(Bucket=bucket_name, Key=key)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    return {"status": "deleted"}


@app.get("/api/v1/storage/{name}/download")
def download_bucket_object(
    name: str,
    key: str,
    request: Request,
    x_dcp_project: str | None = Header(default=None, alias="X-DCP-Project"),
    project: str | None = None,
) -> StreamingResponse:
    user = current_user(request)
    project_id = (x_dcp_project or project or "").strip()
    if not project_id:
        raise HTTPException(status_code=400, detail="プロジェクトを選択してください")
    client, bucket_name = _s3_client(user["id"], project_id, name)
    try:
        resp = client.get_object(Bucket=bucket_name, Key=key)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    content_type = resp.get("ContentType", "application/octet-stream")
    filename = key.split("/")[-1]
    encoded = quote(filename, safe="")
    return StreamingResponse(
        resp["Body"].iter_chunks(),
        media_type=content_type,
        headers={"Content-Disposition": f"attachment; filename*=UTF-8''{encoded}"},
    )


@app.websocket("/api/v1/compute/{name}/console")
async def compute_console(websocket: WebSocket, name: str) -> None:
    import websockets

    session_token = websocket.cookies.get(session_cookie_name(), "").strip()
    project_id = (websocket.query_params.get("projectId") or "").strip()
    if not session_token:
        await websocket.close(code=4401, reason="ログインが必要です")
        return
    if not project_id:
        await websocket.close(code=4400, reason="プロジェクトを選択してください")
        return

    try:
        user = current_user_from_session(session_token)
        resolved = resolve_compute_machine(user["id"], project_id, name)
    except HTTPException as exc:
        await websocket.close(code=4404 if exc.status_code == 404 else 4400, reason=str(exc.detail))
        return
    except RuntimeError as exc:
        await websocket.close(code=4502, reason=str(exc))
        return

    namespace = resolved["namespace"]
    resource_name = compute_machine_resource_name(user["id"], project_id, name)
    upstream_url = (
        "wss://kubernetes.default.svc"
        f"/apis/subresources.kubevirt.io/v1/namespaces/{quote(namespace)}/virtualmachineinstances/{quote(resource_name)}/console"
    )
    ca_path = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
    token_path = "/var/run/secrets/kubernetes.io/serviceaccount/token"
    try:
        with open(token_path, "r", encoding="utf-8") as handle:
            token = handle.read().strip()
        ca_context = ssl.create_default_context(cafile=ca_path)
    except OSError as exc:
        await websocket.close(code=4500, reason=str(exc))
        return

    await websocket.accept()
    try:
        async with websockets.connect(
            upstream_url,
            additional_headers={"Authorization": f"Bearer {token}"},
            subprotocols=["v5.channel.k8s.io", "v4.channel.k8s.io", "v3.channel.k8s.io", "v2.channel.k8s.io", "channel.k8s.io"],
            ssl=ca_context,
            ping_interval=None,
            close_timeout=5,
            max_size=None,
        ) as upstream:
            async def forward_client_to_upstream() -> None:
                try:
                    while True:
                        message = await websocket.receive()
                        if message["type"] == "websocket.disconnect":
                            break
                        text = message.get("text")
                        if text is not None:
                            await upstream.send(text.encode("utf-8"))
                            continue
                        data = message.get("bytes")
                        if data is not None:
                            await upstream.send(data)
                except WebSocketDisconnect:
                    return

            async def forward_upstream_to_client() -> None:
                try:
                    async for message in upstream:
                        if isinstance(message, bytes):
                            payload = message
                            if not payload:
                                continue
                            await websocket.send_bytes(payload)
                        else:
                            await websocket.send_text(message)
                except websockets.ConnectionClosed:
                    return

            client_task = asyncio.create_task(forward_client_to_upstream())
            upstream_task = asyncio.create_task(forward_upstream_to_client())
            done, pending = await asyncio.wait(
                {client_task, upstream_task},
                return_when=asyncio.FIRST_COMPLETED,
            )
            for task in pending:
                task.cancel()
            await asyncio.gather(*pending, return_exceptions=True)
            for task in done:
                task.result()
    except Exception as exc:
        if websocket.client_state.name != "DISCONNECTED":
            await websocket.send_text(f"\r\n[console disconnected] {exc}\r\n")
            await websocket.close(code=1011, reason=str(exc))
