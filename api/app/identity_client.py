from __future__ import annotations

import os
from typing import Any

import grpc

from generated import identity_pb2


def env(key: str, fallback: str) -> str:
    value = os.getenv(key)
    return value if value else fallback


def identity_grpc_addr() -> str:
    return env("DCLD_IDENTITY_GRPC_ADDR", "localhost:8083")


def _rpc_error_message(error: grpc.RpcError) -> str:
    details = error.details() if hasattr(error, "details") else ""
    return details or error.__class__.__name__


class IdentityClient:
    def __init__(self, channel: grpc.Channel) -> None:
        self._me = channel.unary_unary(
            "/dcloud.identity.v1.IdentityService/Me",
            request_serializer=identity_pb2.MeRequest.SerializeToString,
            response_deserializer=identity_pb2.MeResponse.FromString,
        )
        self._login = channel.unary_unary(
            "/dcloud.identity.v1.IdentityService/Login",
            request_serializer=identity_pb2.LoginRequest.SerializeToString,
            response_deserializer=identity_pb2.AuthResponse.FromString,
        )
        self._register = channel.unary_unary(
            "/dcloud.identity.v1.IdentityService/Register",
            request_serializer=identity_pb2.RegisterRequest.SerializeToString,
            response_deserializer=identity_pb2.AuthResponse.FromString,
        )
        self._logout = channel.unary_unary(
            "/dcloud.identity.v1.IdentityService/Logout",
            request_serializer=identity_pb2.LogoutRequest.SerializeToString,
            response_deserializer=identity_pb2.LogoutResponse.FromString,
        )

    @classmethod
    def new(cls) -> "IdentityClient":
        return cls(grpc.insecure_channel(identity_grpc_addr()))

    def me(self, session_token: str) -> dict[str, Any]:
        try:
            response = self._me(identity_pb2.MeRequest(session_token=session_token))
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._user_to_dict(response.user)

    def login(self, username: str, password: str) -> dict[str, Any]:
        try:
            response = self._login(
                identity_pb2.LoginRequest(username=username, password=password)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._auth_to_dict(response)

    def register(
        self,
        username: str,
        password: str,
        email: str = "",
        name: str = "",
    ) -> dict[str, Any]:
        try:
            response = self._register(
                identity_pb2.RegisterRequest(
                    username=username,
                    password=password,
                    email=email,
                    name=name,
                )
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._auth_to_dict(response)

    def logout(self, session_token: str) -> None:
        try:
            self._logout(identity_pb2.LogoutRequest(session_token=session_token))
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    @staticmethod
    def _auth_to_dict(response: identity_pb2.AuthResponse) -> dict[str, Any]:
        return {
            "user": IdentityClient._user_to_dict(response.user),
            "sessionToken": response.session.token,
            "sessionExpiresAt": response.session.expires_at,
        }

    @staticmethod
    def _user_to_dict(user: identity_pb2.User) -> dict[str, Any]:
        return {
            "id": user.id,
            "username": user.username,
            "email": user.email or None,
            "name": user.name or None,
            "createdAt": user.created_at,
            "updatedAt": user.updated_at,
        }

    @staticmethod
    def _map_error(error: grpc.RpcError) -> Exception:
        code = error.code() if hasattr(error, "code") else None
        message = _rpc_error_message(error)
        if code in (grpc.StatusCode.INVALID_ARGUMENT, grpc.StatusCode.ALREADY_EXISTS):
            return ValueError(message)
        if code in (grpc.StatusCode.NOT_FOUND, grpc.StatusCode.UNAUTHENTICATED):
            return PermissionError(message)
        return RuntimeError(message)
