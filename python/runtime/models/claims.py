from typing import List

from pydantic import BaseModel


class Evidence(BaseModel):
    source: str
    uri: str
    title: str
    snippet: str
    region: str
    version: str
    published_at: str


class Claim(BaseModel):
    text: str
    evidence: List[Evidence]
    degraded: bool = False
