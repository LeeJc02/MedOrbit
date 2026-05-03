import statistics
from typing import Any, Dict, List

from evals.cases import GoldenCase


THRESHOLDS = {
    "recall@5": 0.80,
    "precision@5": 0.60,
    "evidence_coverage": 1.0,
    "unsupported_draft_rate": 0.0,
}


def compute_case_metrics(case: GoldenCase, result: Dict[str, Any], latency_ms: float, k: int = 5) -> Dict[str, Any]:
    retrieved_ids = _retrieved_evidence_ids(result)
    expected = case.expected_evidence_ids
    top_k = retrieved_ids[:k]
    relevant = set(expected)
    hits = [evidence_id for evidence_id in top_k if evidence_id in relevant]
    forbidden_terms = [term for term in case.must_not_contain if term in result.get("draft", "")]

    recall = len(set(hits)) / len(relevant) if relevant else 1.0
    precision = len(hits) / k if k else 0.0
    mrr = _mrr(retrieved_ids, relevant)

    claims = result.get("claims", [])
    supported_claims = sum(1 for claim in claims if claim.get("evidence"))
    unsupported_claims = len(claims) - supported_claims

    failures = []
    missing = [evidence_id for evidence_id in expected if evidence_id not in retrieved_ids]
    if missing:
        failures.append({"type": "missing_evidence", "evidence_ids": missing})
    if forbidden_terms:
        failures.append({"type": "unsupported_draft", "terms": forbidden_terms})
    if result.get("risk_level") != case.expected_risk_level:
        failures.append(
            {
                "type": "risk_mismatch",
                "expected": case.expected_risk_level,
                "actual": result.get("risk_level"),
            }
        )
    if unsupported_claims:
        failures.append({"type": "claim_without_evidence", "count": unsupported_claims})

    return {
        "case_id": case.case_id,
        "recall@5": recall,
        "precision@5": precision,
        "mrr": mrr,
        "retrieved_evidence_ids": retrieved_ids,
        "expected_evidence_ids": expected,
        "risk_level": result.get("risk_level"),
        "expected_risk_level": case.expected_risk_level,
        "forbidden_terms": forbidden_terms,
        "claim_count": len(claims),
        "supported_claim_count": supported_claims,
        "unsupported_claim_count": unsupported_claims,
        "latency_ms": latency_ms,
        "failures": failures,
    }


def aggregate_metrics(case_metrics: List[Dict[str, Any]]) -> Dict[str, Any]:
    if not case_metrics:
        return {
            "recall@5": 0.0,
            "precision@5": 0.0,
            "mrr": 0.0,
            "evidence_coverage": 0.0,
            "unsupported_draft_rate": 0.0,
            "latency_p50_ms": 0.0,
            "latency_p95_ms": 0.0,
            "failures": [],
        }

    total_claims = sum(item["claim_count"] for item in case_metrics)
    supported_claims = sum(item["supported_claim_count"] for item in case_metrics)
    unsupported_drafts = sum(1 for item in case_metrics if item["forbidden_terms"])
    latencies = sorted(item["latency_ms"] for item in case_metrics)

    failures = []
    for item in case_metrics:
        if item["failures"]:
            failures.append({"case_id": item["case_id"], "failures": item["failures"]})

    return {
        "recall@5": statistics.fmean(item["recall@5"] for item in case_metrics),
        "precision@5": statistics.fmean(item["precision@5"] for item in case_metrics),
        "mrr": statistics.fmean(item["mrr"] for item in case_metrics),
        "evidence_coverage": supported_claims / total_claims if total_claims else 1.0,
        "unsupported_draft_rate": unsupported_drafts / len(case_metrics),
        "latency_p50_ms": _percentile(latencies, 50),
        "latency_p95_ms": _percentile(latencies, 95),
        "failures": failures,
    }


def threshold_failures(metrics: Dict[str, Any]) -> List[str]:
    failures = []
    for metric, threshold in THRESHOLDS.items():
        value = metrics[metric]
        if metric == "unsupported_draft_rate":
            if value > threshold:
                failures.append(f"{metric}={value:.3f} > {threshold:.3f}")
        elif value < threshold:
            failures.append(f"{metric}={value:.3f} < {threshold:.3f}")
    return failures


def _retrieved_evidence_ids(result: Dict[str, Any]) -> List[str]:
    evidence_ids = []
    seen = set()
    for claim in result.get("claims", []):
        for evidence in claim.get("evidence", []):
            evidence_id = evidence.get("uri")
            if evidence_id and evidence_id not in seen:
                seen.add(evidence_id)
                evidence_ids.append(evidence_id)
    return evidence_ids


def _mrr(retrieved_ids: List[str], relevant: set[str]) -> float:
    for idx, evidence_id in enumerate(retrieved_ids, start=1):
        if evidence_id in relevant:
            return 1 / idx
    return 0.0


def _percentile(values: List[float], percentile: int) -> float:
    if not values:
        return 0.0
    if len(values) == 1:
        return values[0]
    rank = (len(values) - 1) * percentile / 100
    lower = int(rank)
    upper = min(lower + 1, len(values) - 1)
    weight = rank - lower
    return values[lower] * (1 - weight) + values[upper] * weight

