from __future__ import annotations

import asyncio
import ssl
import os
import time
from hashlib import sha256
from typing import Any
from urllib.parse import quote

from fastapi import FastAPI, Header, HTTPException, Request
from fastapi import WebSocket, WebSocketDisconnect
from fastapi.responses import RedirectResponse
from fastapi import Response
from app.routes.project import router as project_router
from app.identity_client import IdentityClient
from app.compute_client import ComputeClient
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
    user = current_user(request)
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
        operation_id = app.state.container_client.delete_service(user["id"], project_id, name)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except KeyError as exc:
        raise HTTPException(status_code=404, detail="サービスが見つかりません") from exc
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc
    return {"status": "deleting", "operationId": operation_id}


@app.get("/api/v1/operations/{operation_id}")
def get_operation(operation_id: str, request: Request) -> dict[str, str]:
    current_user(request)
    for client in [app.state.container_client, app.state.compute_client]:
        try:
            return client.get_operation(operation_id)
        except KeyError:
            continue
        except RuntimeError as exc:
            raise HTTPException(status_code=502, detail=str(exc)) from exc
    raise HTTPException(status_code=404, detail="オペレーションが見つかりません")


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
