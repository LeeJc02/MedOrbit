import json

import grpc
from grpc_health.v1 import health_pb2
from grpc_health.v1 import health_pb2_grpc

import server as runtime_server
import orchestrator_pb2
import orchestrator_pb2_grpc
from agents.graph import AgentRuntimeConfig
from audit.logger import AuditLogger
from memory import MemoryStore
from models.claims import Evidence


def test_runtime_grpc_addr_uses_env(monkeypatch):
    monkeypatch.setenv("DDI_RUNTIME_GRPC_ADDR", "127.0.0.1:50052")
    assert runtime_server.runtime_grpc_addr() == "127.0.0.1:50052"


def test_grpc_run_session_and_replay_returns_runtime_events():
    grpc_server, addr = start_test_server()
    try:
        channel = grpc.insecure_channel(addr)
        stub = orchestrator_pb2_grpc.OrchestratorStub(channel)

        resp = stub.RunSession(
            orchestrator_pb2.RunSessionRequest(
                session_id="grpc-session",
                user_id="u1",
                locale="zh-CN",
                input_text="aspirin warfarin",
            ),
            timeout=3,
        )

        assert resp.session_id == "grpc-session"
        assert resp.claims
        assert resp.risk_level == "MEDIUM"
        assert resp.draft
        assert resp.followups == []

        replay = stub.Replay(
            orchestrator_pb2.ReplayRequest(session_id="grpc-session"),
            timeout=3,
        )
        assert any(event.startswith("runtime.request:") for event in replay.events)
        response_events = [
            event for event in replay.events if event.startswith("runtime.response:")
        ]
        assert response_events
        payload = json.loads(response_events[0].split(":", 1)[1])
        assert payload["claims_count"] == 3
        assert payload["evidence_ids"] == ["fixture://ddi/aspirin-warfarin"]
    finally:
        grpc_server.stop(0)


def test_runtime_health_service_returns_serving():
    grpc_server, addr = start_test_server()
    try:
        channel = grpc.insecure_channel(addr)
        stub = health_pb2_grpc.HealthStub(channel)

        resp = stub.Check(
            health_pb2.HealthCheckRequest(service=runtime_server.HEALTH_SERVICE_NAME),
            timeout=3,
        )

        assert resp.status == health_pb2.HealthCheckResponse.SERVING
    finally:
        grpc_server.stop(0)


def test_service_degrades_claims_without_evidence(monkeypatch):
    def fake_graph(payload, config=None):
        return {
            "claims": [{"text": "unsupported claim", "evidence": []}],
            "risk_level": "UNKNOWN",
            "followups": [],
            "draft": "local draft",
        }

    monkeypatch.setattr(runtime_server, "run_agent_graph", fake_graph)
    service = runtime_server.OrchestratorService(AuditLogger())
    resp = service.RunSession(
        orchestrator_pb2.RunSessionRequest(
            session_id="degraded",
            user_id="u1",
            input_text="unknown",
        ),
        None,
    )

    assert len(resp.claims) == 1
    assert resp.claims[0].degraded is True
    assert "无诊断效力" in resp.claims[0].text


def start_test_server():
    audit_logger = AuditLogger()
    config = AgentRuntimeConfig(
        retriever=lambda rag_name, query: [fixture_evidence()],
        memory=MemoryStore(),
    )
    grpc_server = runtime_server.create_server(
        audit_logger=audit_logger,
        runtime_config=config,
    )
    port = runtime_server.bind_server(grpc_server, "127.0.0.1:0")
    grpc_server.start()
    return grpc_server, f"127.0.0.1:{port}"


def fixture_evidence() -> Evidence:
    return Evidence(
        source="fixture",
        uri="fixture://ddi/aspirin-warfarin",
        title="Aspirin warfarin interaction",
        snippet="Combined use may increase bleeding risk.",
        region="US",
        version="2026.05",
        published_at="2026-05-01",
    )
