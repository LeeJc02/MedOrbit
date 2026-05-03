from pathlib import Path
import sys


_RUNTIME = Path(__file__).resolve().parents[1] / "python" / "runtime"
_RUNTIME_EVALS = _RUNTIME / "evals"
if _RUNTIME.exists() and str(_RUNTIME) not in sys.path:
    sys.path.insert(0, str(_RUNTIME))
if _RUNTIME_EVALS.exists():
    __path__.append(str(_RUNTIME_EVALS))
