from agents.graph import AgentRuntimeConfig, run_agent_graph
from memory import MemoryStore


def test_empty_input_returns_followups_without_evidence():
    config = AgentRuntimeConfig(
        retriever=lambda rag_name, query: [],
        memory=MemoryStore(),
    )

    result = run_agent_graph(
        {
            "session_id": "empty-input",
            "input_text": "  ",
            "locale": "zh-CN",
        },
        config=config,
    )

    assert result["claims"] == []
    assert result["risk_level"] == "UNKNOWN"
    assert result["followups"]
    assert result["draft"] == "需要补充信息后再给出方向性建议。"


def test_draft_includes_evidence_reference():
    from models.claims import Evidence

    evidence = Evidence(
        source="fixture",
        uri="fixture://ddi/example",
        title="Example source",
        snippet="Snippet",
        region="US",
        version="2026.05",
        published_at="2026-05-01",
    )

    config = AgentRuntimeConfig(
        retriever=lambda rag_name, query: [evidence],
        memory=MemoryStore(),
    )

    result = run_agent_graph(
        {
            "session_id": "evidence-ref",
            "input_text": "test",
            "locale": "zh-CN",
        },
        config=config,
    )

    assert result["draft"] == "基于现有证据生成初步草稿。证据：fixture://ddi/example。"
