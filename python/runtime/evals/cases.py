import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, List


EVAL_DIR = Path(__file__).resolve().parent


@dataclass(frozen=True)
class GoldenCase:
    case_id: str
    query: str
    expected_sources: List[str]
    expected_evidence_ids: List[str]
    relevant_terms: List[str]
    must_not_contain: List[str]
    expected_risk_level: str


def load_golden_cases(path: Path | None = None) -> List[GoldenCase]:
    raw_cases = _load_json(path or EVAL_DIR / "golden_cases.json")
    return [GoldenCase(**case) for case in raw_cases]


def load_fixture_evidence(path: Path | None = None) -> List[Dict[str, Any]]:
    return _load_json(path or EVAL_DIR / "fixture_evidence.json")


def _load_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)

