import math
from typing import Any, Dict, Iterable, List

from models.claims import Evidence


def build_vocabulary(rows: Iterable[Dict[str, Any]]) -> List[str]:
    terms = []
    for row in rows:
        terms.extend(row.get("terms", []))
    return sorted(set(terms))


def term_embedding(text: str, vocabulary: List[str]) -> List[float]:
    vector = [1.0 if term in text else 0.0 for term in vocabulary]
    norm = math.sqrt(sum(value * value for value in vector))
    if norm == 0:
        return [0.0 for _ in vector]
    return [value / norm for value in vector]


def row_to_evidence(row: Dict[str, Any]) -> Evidence:
    return Evidence(
        source=row["source"],
        uri=row["uri"],
        title=row["title"],
        snippet=row["snippet"],
        region=row["region"],
        version=row["version"],
        published_at=row["published_at"],
    )


class FixtureRetriever:
    def __init__(self, rows: List[Dict[str, Any]], top_k: int = 5) -> None:
        self.rows = rows
        self.top_k = top_k

    def __call__(self, rag_name: str, query: str) -> List[Evidence]:
        if not query.strip():
            return []

        matches = []
        for row in self.rows:
            if row["rag_name"] != rag_name:
                continue
            score = sum(1 for term in row.get("terms", []) if term in query)
            if score > 0:
                matches.append((score, row["evidence_id"], row))

        matches.sort(key=lambda item: (-item[0], item[1]))
        return [row_to_evidence(row) for _, _, row in matches[: self.top_k]]


def evidence_bound_draft(claims, followups) -> str:
    if followups:
        return "需要补充信息后再给出方向性建议。"
    lines = ["基于已检索证据生成初步草稿："]
    for claim in claims:
        evidence_ids = ", ".join(evidence.uri for evidence in claim.evidence)
        lines.append(f"{claim.text}（证据：{evidence_ids}）")
    return "\n".join(lines)

