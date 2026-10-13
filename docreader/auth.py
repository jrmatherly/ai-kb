"""gRPC TLS and authentication.

Environment variables:
    TLS:
        GRPC_TLS_ENABLED: enable TLS (true/false), default false
        GRPC_TLS_CERT: TLS certificate file path (required when GRPC_TLS_ENABLED=true)
        GRPC_TLS_KEY: TLS private key file path (required when GRPC_TLS_ENABLED=true)
        GRPC_TLS_CA: CA certificate path
        GRPC_MTLS_REQUIRE_CLIENT_CERT: true enables mTLS and requires clients
            to present a certificate signed by GRPC_TLS_CA. When unset, mTLS
            is on if GRPC_TLS_CA is set (kept for backward compatibility).

    Authentication:
        GRPC_AUTH_TOKEN: authentication token; setting it enables authentication

Note: when GRPC_TLS_ENABLED=true and any TLS setting is missing or fails to
load, this module raises so startup fails fast instead of silently falling
back to plaintext.
"""

import hmac
import logging
import os
from typing import Optional

import grpc

logger = logging.getLogger(__name__)


class TLSConfigError(RuntimeError):
    """TLS configuration error, raised to fail fast."""


def _env_bool(name: str, default: bool = False) -> bool:
    raw = os.getenv(name)
    if raw is None:
        return default
    return raw.strip().lower() in ("true", "1", "yes", "on")


def load_tls_credentials() -> Optional[grpc.ServerCredentials]:
    """Build server-side TLS credentials.

    Returns None when GRPC_TLS_ENABLED=false. When true, an invalid configuration
    raises TLSConfigError and the caller decides whether to abort startup.
    """
    if not _env_bool("GRPC_TLS_ENABLED", False):
        logger.info("TLS disabled (GRPC_TLS_ENABLED is not 'true')")
        return None

    cert_path = os.getenv("GRPC_TLS_CERT")
    key_path = os.getenv("GRPC_TLS_KEY")

    if not cert_path or not key_path:
        raise TLSConfigError(
            "GRPC_TLS_ENABLED=true but GRPC_TLS_CERT/GRPC_TLS_KEY not set; "
            "refusing to start in plaintext mode"
        )

    try:
        with open(cert_path, "rb") as f:
            cert_chain = f.read()
        with open(key_path, "rb") as f:
            private_key = f.read()
    except OSError as e:
        raise TLSConfigError(f"failed to read TLS cert/key: {e}") from e

    ca_path = os.getenv("GRPC_TLS_CA")
    require_client_auth = _env_bool(
        "GRPC_MTLS_REQUIRE_CLIENT_CERT",
        default=bool(ca_path),
    )

    if require_client_auth and not ca_path:
        raise TLSConfigError(
            "GRPC_MTLS_REQUIRE_CLIENT_CERT=true requires GRPC_TLS_CA to be set"
        )

    if ca_path:
        try:
            with open(ca_path, "rb") as f:
                ca_cert = f.read()
        except OSError as e:
            raise TLSConfigError(f"failed to read CA cert: {e}") from e
        credentials = grpc.ssl_server_credentials(
            [(private_key, cert_chain)],
            root_certificates=ca_cert,
            require_client_auth=require_client_auth,
        )
        if require_client_auth:
            logger.info("TLS enabled with mTLS (mutual authentication)")
        else:
            logger.info("TLS enabled with CA configured (client auth optional)")
    else:
        credentials = grpc.ssl_server_credentials([(private_key, cert_chain)])
        logger.info("TLS enabled (1-way)")

    return credentials


# Standard gRPC health check paths. They bypass auth so K8s/Docker probes work.
_HEALTH_METHODS = frozenset(
    {
        "/grpc.health.v1.Health/Check",
        "/grpc.health.v1.Health/Watch",
    }
)


def _make_abort_handler(original: Optional[grpc.RpcMethodHandler]) -> grpc.RpcMethodHandler:
    """Build an auth-failure handler that matches the original handler's RPC kind.

    If the original handler is unavailable (should not happen, kept as a fallback),
    return unary_unary. Returning None instead of set_code would also work, but an
    explicit abort makes the framework finish with UNAUTHENTICATED, and matching
    the kind keeps grpc from raising INTERNAL.
    """
    def _abort(_request, context):
        context.abort(
            grpc.StatusCode.UNAUTHENTICATED,
            "Invalid or missing authentication token",
        )

    def _abort_stream(_request, context):
        context.abort(
            grpc.StatusCode.UNAUTHENTICATED,
            "Invalid or missing authentication token",
        )
        return
        yield  # pragma: no cover - make this a generator

    if original is None or original.unary_unary is not None:
        return grpc.unary_unary_rpc_method_handler(
            _abort,
            request_deserializer=getattr(original, "request_deserializer", None),
            response_serializer=getattr(original, "response_serializer", None),
        )
    if original.unary_stream is not None:
        return grpc.unary_stream_rpc_method_handler(
            _abort_stream,
            request_deserializer=original.request_deserializer,
            response_serializer=original.response_serializer,
        )
    if original.stream_unary is not None:
        return grpc.stream_unary_rpc_method_handler(
            _abort,
            request_deserializer=original.request_deserializer,
            response_serializer=original.response_serializer,
        )
    return grpc.stream_stream_rpc_method_handler(
        _abort_stream,
        request_deserializer=original.request_deserializer,
        response_serializer=original.response_serializer,
    )


class AuthInterceptor(grpc.ServerInterceptor):
    """Token authentication interceptor.

    Environment variables:
        GRPC_AUTH_TOKEN: authentication token; setting it enables authentication

    Clients pass the token in metadata:
        - key: "authorization"
        - value: "Bearer <token>" or just "<token>"
    """

    def __init__(self) -> None:
        token = os.getenv("GRPC_AUTH_TOKEN") or ""
        self.auth_token: Optional[bytes] = token.encode("utf-8") if token else None
        if self.auth_token:
            if len(self.auth_token) < 16:
                logger.warning(
                    "GRPC_AUTH_TOKEN is shorter than 16 bytes; consider a stronger token"
                )
            logger.info("Token authentication enabled")
        else:
            logger.warning("Token authentication disabled (GRPC_AUTH_TOKEN not set)")

    def intercept_service(self, continuation, handler_call_details):
        if not self.auth_token:
            return continuation(handler_call_details)

        method = handler_call_details.method
        if method in _HEALTH_METHODS:
            return continuation(handler_call_details)

        metadata = dict(handler_call_details.invocation_metadata or [])
        raw = metadata.get("authorization", "") or ""
        if raw.startswith("Bearer "):
            raw = raw[7:]
        token_bytes = raw.encode("utf-8")

        if not hmac.compare_digest(token_bytes, self.auth_token):
            logger.warning("Authentication failed for method: %s", method)
            original = continuation(handler_call_details)
            return _make_abort_handler(original)

        return continuation(handler_call_details)
