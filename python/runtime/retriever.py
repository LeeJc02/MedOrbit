from typing import List

from models.claims import Evidence


def hybrid_retrieve(rag_name: str, query: str) -> List[Evidence]:
    if not query:
        return []
    return [
        Evidence(
            source=rag_name,
            uri="local://placeholder",
            title="权威来源占位",
            snippet="检索命中占位内容",
            region="CN",
            version="v0",
            published_at="",
        )
    ]
