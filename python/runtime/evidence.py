from typing import List, Dict


def enforce_evidence(claims: List[Dict]) -> List[Dict]:
    normalized = []
    for claim in claims:
        evidence = claim.get("evidence") or []
        degraded = claim.get("degraded", False)
        if not evidence:
            degraded = True
            claim = {
                "text": f"{claim.get('text', '')}（仅 AI 推测、无诊断效力）",
                "evidence": [],
                "degraded": degraded,
            }
        else:
            claim = {
                "text": claim.get("text", ""),
                "evidence": evidence,
                "degraded": degraded,
            }
        normalized.append(claim)
    return normalized
