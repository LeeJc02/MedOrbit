import json
import os
import sys
from concurrent import futures

import grpc
from grpc_health.v1 import health
from grpc_health.v1 import health_pb2
from grpc_health.v1 import health_pb2_grpc

GEN_DIR = os.path.join(os.path.dirname(__file__), "gen")
if GEN_DIR not in sys.path:
    sys.path.insert(0, GEN_DIR)

import orchestrator_pb2  # noqa: E402
import orchestrator_pb2_grpc  # noqa: E402
from agents.graph import AgentRuntimeConfig, run_agent_graph
from audit.logger import AuditLogger
from evidence import enforce_evidence


DEFAULT_GRPC_ADDR = "[::]:50051"
HEALTH_SERVICE_NAME = "ddi.orchestrator.v1.Orchestrator"


class OrchestratorService(orchestrator_pb2_grpc.OrchestratorServicer):
    def __init__(
        self,
        audit_logger: AuditLogger,
        runtime_config: AgentRuntimeConfig | None = None,
    ) -> None:
        self.audit_logger = audit_logger
        self.runtime_config = runtime_config or AgentRuntimeConfig()

    def RunSession(self, request, context):
        payload = {
            "session_id": request.session_id,
            "user_id": request.user_id,
            "locale": request.locale,
            "input_text": request.input_text,
            "metadata": dict(request.metadata),
        }
        self.audit_logger.log_event(
            request.session_id, "runtime.request", json.dumps(payload)
        )

        result = run_agent_graph(payload, config=self.runtime_config)
        claims = enforce_evidence(result["claims"])
        evidence_ids = collect_evidence_ids(claims)

        response = orchestrator_pb2.RunSessionResponse(
            session_id=request.session_id,
            claims=[
                orchestrator_pb2.Claim(
                    text=claim["text"],
                    degraded=claim["degraded"],
                    evidence=[
                        orchestrator_pb2.Evidence(**ev) for ev in claim["evidence"]
                    ],
                )
                for claim in claims
            ],
            draft=result.get("draft", ""),
            risk_level=result.get("risk_level", "UNKNOWN"),
            followups=result.get("followups", []),
        )

        self.audit_logger.log_event(
            request.session_id,
            "runtime.response",
            json.dumps(
                {
                    "claims_count": len(claims),
                    "risk_level": response.risk_level,
                    "followups": list(response.followups),
                    "evidence_ids": evidence_ids,
                }
            ),
        )
        return response

    def Replay(self, request, context):
        events = self.audit_logger.replay(request.session_id)
        return orchestrator_pb2.ReplayResponse(session_id=request.session_id, events=events)


def serve() -> None:
    addr = runtime_grpc_addr()
    server = create_server()
    bind_server(server, addr)
    server.start()
    print(f"runtime gRPC listening on {addr}", file=sys.stderr)
    server.wait_for_termination()


def create_server(
    audit_logger: AuditLogger | None = None,
    runtime_config: AgentRuntimeConfig | None = None,
) -> grpc.Server:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    audit_logger = audit_logger or AuditLogger()
    orchestrator_pb2_grpc.add_OrchestratorServicer_to_server(
        OrchestratorService(audit_logger, runtime_config=runtime_config), server
    )
    health_servicer = health.HealthServicer()
    health_servicer.set(HEALTH_SERVICE_NAME, health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)
    return server


def bind_server(server: grpc.Server, addr: str) -> int:
    port = server.add_insecure_port(addr)
    if port == 0:
        raise RuntimeError(f"failed to bind DDI runtime gRPC address {addr}")
    return port


def runtime_grpc_addr() -> str:
    return os.getenv("DDI_RUNTIME_GRPC_ADDR") or DEFAULT_GRPC_ADDR


def collect_evidence_ids(claims) -> list[str]:
    ids = []
    seen = set()
    for claim in claims:
        for ev in claim.get("evidence", []):
            evidence_id = ev.get("uri") or f"{ev.get('source', '')}:{ev.get('title', '')}"
            if evidence_id and evidence_id not in seen:
                ids.append(evidence_id)
                seen.add(evidence_id)
    return ids


if __name__ == "__main__":
    try:
        serve()
    except RuntimeError as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
