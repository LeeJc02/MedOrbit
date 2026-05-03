import json
from pathlib import Path
from typing import Any, Dict


def write_reports(report: Dict[str, Any], output_dir: Path) -> tuple[Path, Path]:
    output_dir.mkdir(parents=True, exist_ok=True)
    json_path = output_dir / f"agent_eval_{report['mode']}.json"
    md_path = output_dir / f"agent_eval_{report['mode']}.md"
    json_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    md_path.write_text(render_markdown(report), encoding="utf-8")
    return json_path, md_path


def render_markdown(report: Dict[str, Any]) -> str:
    lines = [
        f"# Agent Eval Report: {report['mode']}",
        "",
        f"- Status: {report['status']}",
    ]
    if report.get("skip_reason"):
        lines.append(f"- Skip reason: {report['skip_reason']}")
    lines.extend(
        [
            f"- Recall@5: {report['metrics']['recall@5']:.3f}",
            f"- Precision@5: {report['metrics']['precision@5']:.3f}",
            f"- MRR: {report['metrics']['mrr']:.3f}",
            f"- Evidence coverage: {report['metrics']['evidence_coverage']:.3f}",
            f"- Unsupported draft rate: {report['metrics']['unsupported_draft_rate']:.3f}",
            f"- Latency p50/p95 ms: {report['metrics']['latency_p50_ms']:.2f} / {report['metrics']['latency_p95_ms']:.2f}",
            "",
            "## Cases",
        ]
    )
    for case in report.get("cases", []):
        lines.append(
            f"- `{case['case_id']}` recall={case['recall@5']:.3f} precision={case['precision@5']:.3f} "
            f"mrr={case['mrr']:.3f} latency_ms={case['latency_ms']:.2f}"
        )
    if report.get("threshold_failures"):
        lines.extend(["", "## Threshold Failures"])
        lines.extend(f"- {failure}" for failure in report["threshold_failures"])
    if report["metrics"].get("failures"):
        lines.extend(["", "## Case Failures"])
        for failure in report["metrics"]["failures"]:
            lines.append(f"- `{failure['case_id']}`: {failure['failures']}")
    return "\n".join(lines) + "\n"

