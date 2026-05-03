from typing import Dict, List


class MemoryStore:
    def __init__(self) -> None:
        self.store: Dict[str, List[str]] = {}

    def write(self, session_id: str, key: str, value: str) -> None:
        self.store.setdefault(session_id, []).append(f"{key}:{value}")

    def read(self, session_id: str) -> List[str]:
        return self.store.get(session_id, [])
