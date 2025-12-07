"""
Minimal gRPC JSON client for Python using HTTP/2 (httpx) and dynamic protobuf validation.

Features:
- Accepts a .proto file, compiles descriptors via protoc (grpc_tools), and loads service & message types.
- Validates request and response using google.protobuf dynamic messages (ParseDict/MessageToDict).
- Sends application/grpc+json framed requests over HTTP/2.

Requirements:
    pip install httpx grpcio-tools protobuf
    A protoc compiler must be available (grpc_tools.protoc provides one in-process).
"""
from __future__ import annotations

import json
import os
import tempfile
from dataclasses import dataclass
from typing import Any, Callable, Dict

import httpx
from google.protobuf import descriptor_pb2, descriptor_pool, json_format, message_factory
from grpc_tools import protoc

CONTENT_TYPE = "application/grpc+json"


def _frame_message(data: bytes) -> bytes:
    # 1 byte compressed flag + 4 byte big-endian length + payload
    return bytes([0]) + len(data).to_bytes(4, "big") + data


def _parse_unary_response(buf: bytes) -> Any:
    if len(buf) < 5:
        raise ValueError("grpc: short response")
    flag = buf[0]
    if flag != 0:
        raise ValueError("grpc: compression not supported")
    length = int.from_bytes(buf[1:5], "big")
    if len(buf) < 5 + length:
        raise ValueError("grpc: incomplete message")
    payload = buf[5 : 5 + length]
    return json.loads(payload.decode("utf-8") or "null")


def _build_descriptor_set(proto_path: str, include_paths: list[str] | None = None) -> descriptor_pb2.FileDescriptorSet:
    include_paths = include_paths or [os.path.dirname(proto_path) or "."]
    with tempfile.TemporaryDirectory() as td:
        out_file = os.path.join(td, "desc.pb")
        args = [
            "protoc",
            f"-I{os.path.dirname(proto_path) or '.'}",
            f"--descriptor_set_out={out_file}",
            "--include_imports",
            proto_path,
        ]
        # Invoke protoc via grpc_tools.protoc
        if protoc.main(args) != 0:
            raise RuntimeError("protoc failed building descriptor set")
        with open(out_file, "rb") as f:
            data = f.read()
    fds = descriptor_pb2.FileDescriptorSet()
    fds.ParseFromString(data)
    return fds


@dataclass
class _ServiceInfo:
    pool: descriptor_pool.DescriptorPool
    factory: message_factory.MessageFactory
    pkg: str
    name: str
    methods: Dict[str, tuple]


def _load_service(proto_path: str, service_fqn: str) -> _ServiceInfo:
    fds = _build_descriptor_set(proto_path)
    pool = descriptor_pool.DescriptorPool()
    for fd_proto in fds.file:
        pool.Add(fd_proto)
    factory_inst = message_factory.MessageFactory(pool)

    if "." not in service_fqn:
        raise ValueError("service_fqn must include package, e.g. 'prompt.v1.PromptService'")
    pkg, svc_name = service_fqn.rsplit(".", 1)

    # Find service
    svc_desc = None
    for fd_proto in fds.file:
        if fd_proto.package != pkg:
            continue
        for svc in fd_proto.service:
            if svc.name == svc_name:
                svc_desc = (fd_proto, svc)
                break
        if svc_desc:
            break
    if not svc_desc:
        raise ValueError(f"service not found: {service_fqn}")
    fd_proto, svc = svc_desc

    methods: Dict[str, tuple] = {}
    for m in svc.method:
        # Resolve request/response message descriptors
        req_type = pool.FindMessageTypeByName(m.input_type.lstrip("."))
        res_type = pool.FindMessageTypeByName(m.output_type.lstrip("."))
        methods[m.name] = (req_type, res_type)
    return _ServiceInfo(pool=pool, factory=factory_inst, pkg=pkg, name=svc_name, methods=methods)


class GrpcJsonClient:
    def __init__(self, base_url: str, proto_path: str, service_fqn: str, timeout: float = 30.0):
        self.base_url = base_url.rstrip("/")
        self.svc = _load_service(proto_path, service_fqn)
        self.timeout = timeout
        self._http = httpx.Client(http2=True, timeout=timeout)

    def _method_path(self, method: str) -> str:
        return f"/{self.svc.pkg}.{self.svc.name}/{method}"

    def _validate_request(self, method: str, obj: Dict[str, Any]) -> Any:
        req_desc, _ = self.svc.methods[method]
        msg_cls = self.svc.factory.GetPrototype(req_desc)  # type: ignore
        # ParseDict validates field names/types
        msg = msg_cls()
        json_format.ParseDict(obj or {}, msg)
        return json.loads(json_format.MessageToJson(msg))

    def _validate_response(self, method: str, obj: Dict[str, Any]) -> Any:
        _, res_desc = self.svc.methods[method]
        msg_cls = self.svc.factory.GetPrototype(res_desc)  # type: ignore
        msg = msg_cls()
        json_format.ParseDict(obj or {}, msg)
        return json.loads(json_format.MessageToJson(msg))

    def call(self, method: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        if method not in self.svc.methods:
            raise ValueError(f"unknown method: {method}")
        valid_req = self._validate_request(method, payload)
        body = _frame_message(json.dumps(valid_req).encode("utf-8"))
        path = self._method_path(method)

        headers = {
            "content-type": CONTENT_TYPE,
            "te": "trailers",
            "grpc-accept-encoding": "identity",
            "grpc-encoding": "identity",
        }
        url = f"{self.base_url}{path}"
        # httpx preserves HTTP/2 and should expose trailers as part of extensions
        resp = self._http.post(url, content=body, headers=headers)
        # Extract grpc-status from trailers if present
        status = 0
        message = ""
        trailers = resp.extensions.get("http2", {}).get("trailers") if hasattr(resp, "extensions") else None
        if trailers:
            status = int(trailers.get("grpc-status", ["0"]) [0])
            message = trailers.get("grpc-message", [""])[0]
        # Fallback if server sent headers-only status
        if "grpc-status" in resp.headers:
            status = int(resp.headers.get("grpc-status", "0"))
            message = resp.headers.get("grpc-message", "")
        if status != 0:
            raise RuntimeError(f"grpc error {status}: {message}")

        obj = _parse_unary_response(resp.content)
        valid_res = self._validate_response(method, obj)
        return valid_res

    # Convenience: map RPC methods to callables
    def bind_methods(self) -> Dict[str, Callable[[Dict[str, Any]], Dict[str, Any]]]:
        out: Dict[str, Callable[[Dict[str, Any]], Dict[str, Any]]] = {}
        for m in self.svc.methods.keys():
            out[m] = lambda payload, _m=m: self.call(_m, payload)
        return out


__all__ = ["GrpcJsonClient"]

