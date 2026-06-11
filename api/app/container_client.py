from __future__ import annotations

import os
from typing import Any

import grpc

from generated import container_pb2


def env(key: str, fallback: str) -> str:
    value = os.getenv(key)
    return value if value else fallback


def container_grpc_addr() -> str:
    return env("DCLD_CONTAINER_GRPC_ADDR", "localhost:8082")


def _rpc_error_message(error: grpc.RpcError) -> str:
    details = error.details() if hasattr(error, "details") else ""
    return details or error.__class__.__name__


class ContainerClient:
    def __init__(self, channel: grpc.Channel) -> None:
        self._list_services = channel.unary_unary(
            "/dcloud.container.v1.ContainerService/ListServices",
            request_serializer=container_pb2.ListServicesRequest.SerializeToString,
            response_deserializer=container_pb2.ListServicesResponse.FromString,
        )
        self._deploy_service = channel.unary_unary(
            "/dcloud.container.v1.ContainerService/DeployService",
            request_serializer=container_pb2.DeployServiceRequest.SerializeToString,
            response_deserializer=container_pb2.DeployServiceResponse.FromString,
        )
        self._delete_service = channel.unary_unary(
            "/dcloud.container.v1.ContainerService/DeleteService",
            request_serializer=container_pb2.DeleteServiceRequest.SerializeToString,
            response_deserializer=container_pb2.DeleteServiceResponse.FromString,
        )
        self._get_operation = channel.unary_unary(
            "/dcloud.container.v1.ContainerService/GetOperation",
            request_serializer=container_pb2.GetOperationRequest.SerializeToString,
            response_deserializer=container_pb2.GetOperationResponse.FromString,
        )
        self._set_service_domain = channel.unary_unary(
            "/dcloud.container.v1.ContainerService/SetServiceDomain",
            request_serializer=container_pb2.SetServiceDomainRequest.SerializeToString,
            response_deserializer=container_pb2.SetServiceDomainResponse.FromString,
        )

    @classmethod
    def new(cls) -> "ContainerClient":
        return cls(grpc.insecure_channel(container_grpc_addr()))

    def list_services(self, user_id: str, project_id: str) -> dict[str, Any]:
        try:
            response = self._list_services(
                container_pb2.ListServicesRequest(user_id=user_id, project_id=project_id)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return {
            "userId": response.user_id,
            "projectId": response.project_id,
            "namespace": response.namespace,
            "containers": [self._service_to_dict(service) for service in response.containers],
        }

    def deploy_service(
        self,
        user_id: str,
        project_id: str,
        name: str,
        image: str,
        port: int,
        min_scale: int,
        max_scale: int,
    ) -> dict[str, Any]:
        try:
            response = self._deploy_service(
                container_pb2.DeployServiceRequest(
                    user_id=user_id,
                    project_id=project_id,
                    name=name,
                    image=image,
                    port=port,
                    min_scale=min_scale,
                    max_scale=max_scale,
                )
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._service_to_dict(response.service)

    def delete_service(self, user_id: str, project_id: str, name: str) -> str:
        try:
            response = self._delete_service(
                container_pb2.DeleteServiceRequest(
                    user_id=user_id,
                    project_id=project_id,
                    name=name,
                )
            )
            return response.operation_id
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    def set_service_domain(self, user_id: str, project_id: str, name: str, custom_domain: str) -> dict[str, Any]:
        try:
            response = self._set_service_domain(
                container_pb2.SetServiceDomainRequest(
                    user_id=user_id,
                    project_id=project_id,
                    name=name,
                    custom_domain=custom_domain,
                )
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._service_to_dict(response.service)

    def get_operation(self, operation_id: str) -> dict[str, Any]:
        try:
            response = self._get_operation(
                container_pb2.GetOperationRequest(operation_id=operation_id)
            )
            return {"operationId": response.operation_id, "status": response.status, "error": response.error}
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    @staticmethod
    def _service_to_dict(service: container_pb2.Service) -> dict[str, Any]:
        return {
            "name": service.name,
            "image": service.image,
            "url": service.url,
            "ready": service.ready,
            "reason": service.reason,
            "createdAt": service.created_at,
            "updatedAt": service.updated_at,
            "namespace": service.namespace,
            "projectId": service.project_id,
            "generation": service.generation,
            "customDomain": service.custom_domain or None,
            "domainStatus": service.domain_status or None,
            "domainStatusReason": service.domain_status_reason or None,
        }

    @staticmethod
    def _map_error(error: grpc.RpcError) -> Exception:
        code = error.code() if hasattr(error, "code") else None
        message = _rpc_error_message(error)
        if code == grpc.StatusCode.INVALID_ARGUMENT:
            return ValueError(message)
        if code == grpc.StatusCode.NOT_FOUND:
            return KeyError(message)
        return RuntimeError(message)
