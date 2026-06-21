from __future__ import annotations

import os
from typing import Any

import grpc

from generated import dbaas_pb2


def env(key: str, fallback: str) -> str:
    value = os.getenv(key)
    return value if value else fallback


def dbaas_grpc_addr() -> str:
    return env("DCLD_DBAAS_GRPC_ADDR", "localhost:8086")


def _rpc_error_message(error: grpc.RpcError) -> str:
    details = error.details() if hasattr(error, "details") else ""
    return details or error.__class__.__name__


class DbaasClient:
    def __init__(self, channel: grpc.Channel) -> None:
        self._list_databases = channel.unary_unary(
            "/dcloud.dbaas.v1.DatabaseService/ListDatabases",
            request_serializer=dbaas_pb2.ListDatabasesRequest.SerializeToString,
            response_deserializer=dbaas_pb2.ListDatabasesResponse.FromString,
        )
        self._create_database = channel.unary_unary(
            "/dcloud.dbaas.v1.DatabaseService/CreateDatabase",
            request_serializer=dbaas_pb2.CreateDatabaseRequest.SerializeToString,
            response_deserializer=dbaas_pb2.CreateDatabaseResponse.FromString,
        )
        self._delete_database = channel.unary_unary(
            "/dcloud.dbaas.v1.DatabaseService/DeleteDatabase",
            request_serializer=dbaas_pb2.DeleteDatabaseRequest.SerializeToString,
            response_deserializer=dbaas_pb2.DeleteDatabaseResponse.FromString,
        )
        self._get_database = channel.unary_unary(
            "/dcloud.dbaas.v1.DatabaseService/GetDatabase",
            request_serializer=dbaas_pb2.GetDatabaseRequest.SerializeToString,
            response_deserializer=dbaas_pb2.GetDatabaseResponse.FromString,
        )
        self._get_connection_string = channel.unary_unary(
            "/dcloud.dbaas.v1.DatabaseService/GetConnectionString",
            request_serializer=dbaas_pb2.GetConnectionStringRequest.SerializeToString,
            response_deserializer=dbaas_pb2.GetConnectionStringResponse.FromString,
        )
        self._get_operation = channel.unary_unary(
            "/dcloud.dbaas.v1.DatabaseService/GetOperation",
            request_serializer=dbaas_pb2.GetOperationRequest.SerializeToString,
            response_deserializer=dbaas_pb2.GetOperationResponse.FromString,
        )

    @classmethod
    def new(cls) -> "DbaasClient":
        return cls(grpc.insecure_channel(dbaas_grpc_addr()))

    def list_databases(self, user_id: str, project_id: str) -> dict[str, Any]:
        try:
            response = self._list_databases(
                dbaas_pb2.ListDatabasesRequest(user_id=user_id, project_id=project_id)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return {
            "userId": response.user_id,
            "projectId": response.project_id,
            "databases": [self._db_to_dict(db) for db in response.databases],
        }

    def create_database(
        self,
        user_id: str,
        project_id: str,
        name: str,
        db_type: str,
        version: str,
        cpu: str,
        memory: str,
        storage: str,
    ) -> dict[str, Any]:
        try:
            response = self._create_database(
                dbaas_pb2.CreateDatabaseRequest(
                    user_id=user_id,
                    project_id=project_id,
                    name=name,
                    type=db_type,
                    version=version,
                    cpu=cpu,
                    memory=memory,
                    storage=storage,
                )
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._db_to_dict(response.database)

    def delete_database(self, user_id: str, project_id: str, name: str) -> str:
        try:
            response = self._delete_database(
                dbaas_pb2.DeleteDatabaseRequest(user_id=user_id, project_id=project_id, name=name)
            )
            return response.operation_id
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    def get_database(self, user_id: str, project_id: str, name: str) -> dict[str, Any]:
        try:
            response = self._get_database(
                dbaas_pb2.GetDatabaseRequest(user_id=user_id, project_id=project_id, name=name)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._db_to_dict(response.database)

    def get_connection_string(self, user_id: str, project_id: str, name: str) -> dict[str, Any]:
        try:
            response = self._get_connection_string(
                dbaas_pb2.GetConnectionStringRequest(user_id=user_id, project_id=project_id, name=name)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return {
            "connectionString": response.connection_string,
            "host": response.host,
            "port": response.port,
            "username": response.username,
            "password": response.password,
            "databaseName": response.database_name,
        }

    def get_operation(self, operation_id: str) -> dict[str, Any]:
        try:
            response = self._get_operation(
                dbaas_pb2.GetOperationRequest(operation_id=operation_id)
            )
            return {"operationId": response.operation_id, "status": response.status, "error": response.error}
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    @staticmethod
    def _db_to_dict(db: dbaas_pb2.Database) -> dict[str, Any]:
        return {
            "name": db.name,
            "type": db.type,
            "version": db.version,
            "cpu": db.cpu,
            "memory": db.memory,
            "storage": db.storage,
            "ready": db.ready,
            "status": db.status,
            "createdAt": db.created_at,
            "projectId": db.project_id,
        }

    @staticmethod
    def _map_error(error: grpc.RpcError) -> Exception:
        code = error.code() if hasattr(error, "code") else None
        message = _rpc_error_message(error)
        if code == grpc.StatusCode.INVALID_ARGUMENT:
            return ValueError(message)
        if code == grpc.StatusCode.NOT_FOUND:
            return KeyError(message)
        if code == grpc.StatusCode.ALREADY_EXISTS:
            return KeyError(message)
        return RuntimeError(message)
