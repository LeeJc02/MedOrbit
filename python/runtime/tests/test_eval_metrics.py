from evals.cases import GoldenCase
from evals.metrics import aggregate_metrics, compute_case_metrics, threshold_failures


def test_metrics_formula_for_recall_precision_mrr_and_coverage():
    case = GoldenCase(
        case_id="case-1",
        query="query",
        expected_sources=["RAG1", "RAG2", "RAG3"],
        expected_evidence_ids=["e1", "e2", "e3"],
        relevant_terms=[],
        must_not_contain=["forbidden"],
        expected_risk_level="MEDIUM",
    )
    result = {
        "claims": [
            {"text": "a", "evidence": [{"uri": "e1"}]},
            {"text": "b", "evidence": [{"uri": "e2"}]},
            {"text": "c", "evidence": [{"uri": "noise"}]},
        ],
        "risk_level": "MEDIUM",
        "draft": "supported draft",
    }

    case_metrics = compute_case_metrics(case, result, latency_ms=10.0)
    aggregate = aggregate_metrics([case_metrics])

    assert case_metrics["recall@5"] == 2 / 3
    assert case_metrics["precision@5"] == 2 / 5
    assert case_metrics["mrr"] == 1.0
    assert aggregate["evidence_coverage"] == 1.0
    assert aggregate["unsupported_draft_rate"] == 0.0
    assert threshold_failures(aggregate) == [
        "recall@5=0.667 < 0.800",
        "precision@5=0.400 < 0.600",
    ]

