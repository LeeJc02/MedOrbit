from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Protocol

from memory import MemoryStore
from models.claims import Claim, Evidence
from retriever import hybrid_retrieve


memory_store = MemoryStore()


class MemoryWriter(Protocol):
    def write(self, session_id: str, key: str, value: str) -> None:
        ...


Retriever = Callable[[str, str], List[Evidence]]
LocalDraftGenerator = Callable[[List[Claim], List[str]], str]


@dataclass
class AgentRuntimeConfig:
    retriever: Retriever = hybrid_retrieve
    memory: MemoryWriter = memory_store
    local_draft_generator: LocalDraftGenerator | None = None

    def __post_init__(self) -> None:
        if self.local_draft_generator is None:
            self.local_draft_generator = draft_response


def run_agent_graph(payload: Dict[str, Any], config: AgentRuntimeConfig | None = None) -> Dict[str, Any]:
    config = config or AgentRuntimeConfig()
    session_id = payload["session_id"]
    input_text = payload["input_text"]

    config.memory.write(session_id, "input", input_text)

    normalized = normalize_terms(input_text)
    triage = run_rag_stage("RAG1", "分诊方向：需注意潜在风险", normalized, config.retriever)
    diagnosis = run_rag_stage("RAG2", "诊断方向：需要鉴别其他原因", normalized, config.retriever)
    medication = run_rag_stage("RAG3", "用药安全：请核对相互作用", normalized, config.retriever)

    claims = []
    claims.extend(triage)
    claims.extend(diagnosis)
    claims.extend(medication)

    risk_level = derive_risk(triage)
    followups = collect_followups(normalized, triage)

    draft = config.local_draft_generator(claims, followups)

    return {
        "claims": [c.model_dump() for c in claims],
        "risk_level": risk_level,
        "followups": followups,
        "draft": draft,
    }


def normalize_terms(text: str) -> str:
    return text.strip()


def run_rag_stage(rag_name: str, claim_text: str, query: str, retriever: Retriever) -> List[Claim]:
    evidence = retriever(rag_name, query)
    if not evidence:
        return []
    return [Claim(text=claim_text, evidence=evidence)]


def derive_risk(triage_claims) -> str:
    if not triage_claims:
        return "UNKNOWN"
    return "MEDIUM"


def collect_followups(normalized: str, triage_claims) -> list:
    if triage_claims:
        return []
    return ["请补充症状持续时间", "是否存在既往用药或过敏史"]


def draft_response(claims, followups) -> str:
    if followups:
        return "需要补充信息后再给出方向性建议。"
    references = evidence_references(claims)
    if references:
        return f"基于现有证据生成初步草稿。证据：{'; '.join(references)}。"
    return "基于现有证据生成初步草稿。"


def evidence_references(claims) -> List[str]:
    references = []
    seen = set()
    for claim in claims:
        for ev in claim.evidence:
            ref = ev.uri or f"{ev.source}:{ev.title}"
            if ref and ref not in seen:
                references.append(ref)
                seen.add(ref)
    return references
