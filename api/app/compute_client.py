from __future__ import annotations

import os
from typing import Any

import grpc

from generated import compute_pb2


def env(key: str, fallback: str) -> str:
    value = os.getenv(key)
    return value if value else fallback


def compute_grpc_addr() -> str:
    return env("DCLD_COMPUTE_GRPC_ADDR", "localhost:8084")


def _rpc_error_message(error: grpc.RpcError) -> str:
    details = error.details() if hasattr(error, "details") else ""
    return details or error.__class__.__name__


class ComputeClient:
    def __init__(self, channel: grpc.Channel) -> None:
        self._list_machines = channel.unary_unary(
            "/dcloud.compute.v1.ComputeService/ListMachines",
            request_serializer=compute_pb2.ListMachinesRequest.SerializeToString,
            response_deserializer=compute_pb2.ListMachinesResponse.FromString,
        )
        self._create_machine = channel.unary_unary(
            "/dcloud.compute.v1.ComputeService/CreateMachine",
            request_serializer=compute_pb2.CreateMachineRequest.SerializeToString,
            response_deserializer=compute_pb2.CreateMachineResponse.FromString,
        )
        self._delete_machine = channel.unary_unary(
            "/dcloud.compute.v1.ComputeService/DeleteMachine",
            request_serializer=compute_pb2.DeleteMachineRequest.SerializeToString,
            response_deserializer=compute_pb2.DeleteMachineResponse.FromString,
        )
        self._get_operation = channel.unary_unary(
            "/dcloud.compute.v1.ComputeService/GetOperation",
            request_serializer=compute_pb2.GetOperationRequest.SerializeToString,
            response_deserializer=compute_pb2.GetOperationResponse.FromString,
        )

    @classmethod
    def new(cls) -> "ComputeClient":
        return cls(grpc.insecure_channel(compute_grpc_addr()))

    def list_machines(self, user_id: str, project_id: str) -> dict[str, Any]:
        try:
            response = self._list_machines(
                compute_pb2.ListMachinesRequest(user_id=user_id, project_id=project_id)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return {
            "userId": response.user_id,
            "projectId": response.project_id,
            "namespace": response.namespace,
            "machines": [self._machine_to_dict(machine) for machine in response.machines],
        }

    def create_machine(
        self,
        user_id: str,
        project_id: str,
        name: str,
        image: str,
        cpu: str,
        memory: str,
    ) -> dict[str, Any]:
        try:
            response = self._create_machine(
                compute_pb2.CreateMachineRequest(
                    user_id=user_id,
                    project_id=project_id,
                    name=name,
                    image=image,
                    cpu=cpu,
                    memory=memory,
                )
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._machine_to_dict(response.machine)

    def delete_machine(self, user_id: str, project_id: str, name: str) -> str:
        try:
            response = self._delete_machine(
                compute_pb2.DeleteMachineRequest(
                    user_id=user_id,
                    project_id=project_id,
                    name=name,
                )
            )
            return response.operation_id
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    def get_operation(self, operation_id: str) -> dict[str, Any]:
        try:
            response = self._get_operation(
                compute_pb2.GetOperationRequest(operation_id=operation_id)
            )
            return {"operationId": response.operation_id, "status": response.status, "error": response.error}
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    @staticmethod
    def _machine_to_dict(machine: compute_pb2.Machine) -> dict[str, Any]:
        return {
            "name": machine.name,
            "image": machine.image,
            "cpu": machine.cpu,
            "memory": machine.memory,
            "ready": machine.ready,
            "status": machine.status,
            "reason": machine.reason,
            "createdAt": machine.created_at,
            "updatedAt": machine.updated_at,
            "namespace": machine.namespace,
            "projectId": machine.project_id,
            "generation": machine.generation,
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
