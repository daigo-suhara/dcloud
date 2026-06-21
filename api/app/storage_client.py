from __future__ import annotations

import os
from typing import Any

import grpc

from generated import storage_pb2


def env(key: str, fallback: str) -> str:
    value = os.getenv(key)
    return value if value else fallback


def storage_grpc_addr() -> str:
    return env("DCLD_STORAGE_GRPC_ADDR", "localhost:8085")


def _rpc_error_message(error: grpc.RpcError) -> str:
    details = error.details() if hasattr(error, "details") else ""
    return details or error.__class__.__name__


class StorageClient:
    def __init__(self, channel: grpc.Channel) -> None:
        self._list_buckets = channel.unary_unary(
            "/dcloud.storage.v1.ObjectStorageService/ListBuckets",
            request_serializer=storage_pb2.ListBucketsRequest.SerializeToString,
            response_deserializer=storage_pb2.ListBucketsResponse.FromString,
        )
        self._create_bucket = channel.unary_unary(
            "/dcloud.storage.v1.ObjectStorageService/CreateBucket",
            request_serializer=storage_pb2.CreateBucketRequest.SerializeToString,
            response_deserializer=storage_pb2.CreateBucketResponse.FromString,
        )
        self._delete_bucket = channel.unary_unary(
            "/dcloud.storage.v1.ObjectStorageService/DeleteBucket",
            request_serializer=storage_pb2.DeleteBucketRequest.SerializeToString,
            response_deserializer=storage_pb2.DeleteBucketResponse.FromString,
        )
        self._get_bucket_credentials = channel.unary_unary(
            "/dcloud.storage.v1.ObjectStorageService/GetBucketCredentials",
            request_serializer=storage_pb2.GetBucketCredentialsRequest.SerializeToString,
            response_deserializer=storage_pb2.GetBucketCredentialsResponse.FromString,
        )
        self._get_operation = channel.unary_unary(
            "/dcloud.storage.v1.ObjectStorageService/GetOperation",
            request_serializer=storage_pb2.GetOperationRequest.SerializeToString,
            response_deserializer=storage_pb2.GetOperationResponse.FromString,
        )

    @classmethod
    def new(cls) -> "StorageClient":
        return cls(grpc.insecure_channel(storage_grpc_addr()))

    def list_buckets(self, user_id: str, project_id: str) -> dict[str, Any]:
        try:
            response = self._list_buckets(
                storage_pb2.ListBucketsRequest(user_id=user_id, project_id=project_id)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return {
            "userId": response.user_id,
            "projectId": response.project_id,
            "buckets": [self._bucket_to_dict(b) for b in response.buckets],
        }

    def create_bucket(self, user_id: str, project_id: str, name: str) -> dict[str, Any]:
        try:
            response = self._create_bucket(
                storage_pb2.CreateBucketRequest(user_id=user_id, project_id=project_id, name=name)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        return self._bucket_to_dict(response.bucket)

    def delete_bucket(self, user_id: str, project_id: str, name: str) -> str:
        try:
            response = self._delete_bucket(
                storage_pb2.DeleteBucketRequest(user_id=user_id, project_id=project_id, name=name)
            )
            return response.operation_id
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    def get_bucket_credentials(self, user_id: str, project_id: str, name: str) -> dict[str, Any]:
        try:
            response = self._get_bucket_credentials(
                storage_pb2.GetBucketCredentialsRequest(user_id=user_id, project_id=project_id, name=name)
            )
        except grpc.RpcError as error:
            raise self._map_error(error) from error
        creds = response.credentials
        return {
            "endpoint": creds.endpoint,
            "bucketName": creds.bucket_name,
            "accessKeyId": creds.access_key_id,
            "secretAccessKey": creds.secret_access_key,
        }

    def get_operation(self, operation_id: str) -> dict[str, Any]:
        try:
            response = self._get_operation(
                storage_pb2.GetOperationRequest(operation_id=operation_id)
            )
            return {"operationId": response.operation_id, "status": response.status, "error": response.error}
        except grpc.RpcError as error:
            raise self._map_error(error) from error

    @staticmethod
    def _bucket_to_dict(bucket: storage_pb2.Bucket) -> dict[str, Any]:
        return {
            "name": bucket.name,
            "endpoint": bucket.endpoint,
            "ready": bucket.ready,
            "status": bucket.status,
            "createdAt": bucket.created_at,
            "projectId": bucket.project_id,
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
