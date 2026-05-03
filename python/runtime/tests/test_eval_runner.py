from evals.run_agent_eval import run_integration_eval, run_offline_eval


def test_offline_eval_passes_thresholds():
    report = run_offline_eval()

    assert report["status"] == "passed"
    assert report["threshold_failures"] == []
    assert report["metrics"]["recall@5"] >= 0.8
    assert report["metrics"]["precision@5"] >= 0.6


def test_integration_eval_skips_without_flag(monkeypatch):
    monkeypatch.delenv("DDI_EVAL_INTEGRATION", raising=False)

    report = run_integration_eval()

    assert report["status"] == "skipped"
    assert "DDI_EVAL_INTEGRATION=1" in report["skip_reason"]
