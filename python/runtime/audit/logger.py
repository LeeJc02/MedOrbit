from typing import List


class AuditLogger:
    def __init__(self) -> None:
        self._events = {}

    def log_event(self, session_id: str, event_type: str, payload: str) -> None:
        self._events.setdefault(session_id, []).append(f"{event_type}:{payload}")

    def replay(self, session_id: str) -> List[str]:
        return self._events.get(session_id, [])
