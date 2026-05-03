import argparse
import os
import sys
import time
from pathlib import Path
from typing import Any, Dict, List

from agents.graph import AgentRuntimeConfig, run_agent_graph
from evals.cases import load_fixture_evidence, load_golden_cases
from evals.fixture import FixtureRetriever, evidence_bound_draft
from evals.metrics import aggregate_metrics, compute_case_metrics, threshold_failures
from evals.reporting import write_reports
from memory import MemoryStore


DEFAULT_OUTPUT_DIR = Path(__file__).resolve().parent / "reports"


def main(argv: List[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run local DDI agent evaluations.")
    parser.add_argument("--mode", choices=["offline", "integration"], default="offline")
    parser.add_argument("--fail-on-threshold", action="store_true")
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT_DIR)
    args = parser.parse_args(argv)

    if args.mode == "offline":
        report = run_offline_eval()
    else:
        report = run_integration_eval()

    json_path, md_path = write_reports(report, args.output_dir)
    print(f"status={report['status']} json={json_path} markdown={md_path}")
    if report.get("skip_reason"):
        print(f"skip_reason={report['skip_reason']}")
    for failure in report.get("threshold_failures", []):
        print(f"threshold_failure={failure}")

    if args.fail_on_threshold and report["status"] == "failed":
        return 1
    return 0


def run_offline_eval() -> Dict[str, Any]:
    rows = load_fixture_evidence()
    retriever = FixtureRetriever(rows)
    config = AgentRuntimeConfig(
        retriever=retriever,
        memory=MemoryStore(),
        local_draft_generator=evidence_bound_draft,
    )
    return _run_cases("offline", config)


def run_integration_eval() -> Dict[str, Any]:
    if os.getenv("DDI_EVAL_INTEGRATION") != "1":
        return _skipped_report("integration", "Set DDI_EVAL_INTEGRATION=1 and start Docker Compose services first.")

    from evals.integration_index import DockerIndexRetriever, milvus_available, postgres_available

    pg_ok, pg_reason = postgres_available()
    milvus_ok, milvus_reason = milvus_available()
    if not pg_ok or not milvus_ok:
        return _skipped_report(
            "integration",
            f"Docker services unavailable: postgres={pg_reason}; milvus={milvus_reason}",
        )

    retriever = DockerIndexRetriever(load_fixture_evidence())
    retriever.setup()
    config = AgentRuntimeConfig(
        retriever=retriever,
        memory=MemoryStore(),
        local_draft_generator=evidence_bound_draft,
    )
    return _run_cases("integration", config)


def _run_cases(mode: str, config: AgentRuntimeConfig) -> Dict[str, Any]:
    case_metrics = []
    for case in load_golden_cases():
        payload = {
            "session_id": f"eval-{mode}-{case.case_id}",
            "user_id": "eval",
            "locale": "zh-CN",
            "input_text": case.query,
            "metadata": {"eval_mode": mode},
        }
        started = time.perf_counter()
        result = run_agent_graph(payload, config=config)
        latency_ms = (time.perf_counter() - started) * 1000
        case_metrics.append(compute_case_metrics(case, result, latency_ms))

    metrics = aggregate_metrics(case_metrics)
    failures = threshold_failures(metrics)
    return {
        "mode": mode,
        "status": "failed" if failures else "passed",
        "metrics": metrics,
        "threshold_failures": failures,
        "cases": case_metrics,
    }


def _skipped_report(mode: str, reason: str) -> Dict[str, Any]:
    return {
        "mode": mode,
        "status": "skipped",
        "skip_reason": reason,
        "metrics": {
            "recall@5": 0.0,
            "precision@5": 0.0,
            "mrr": 0.0,
            "evidence_coverage": 0.0,
            "unsupported_draft_rate": 0.0,
            "latency_p50_ms": 0.0,
            "latency_p95_ms": 0.0,
            "failures": [],
        },
        "threshold_failures": [],
        "cases": [],
    }


if __name__ == "__main__":
    sys.exit(main())
