import pytest

from evals.run_agent_eval import run_integration_eval


@pytest.mark.integration
def test_integration_eval_uses_docker_services_when_enabled():
    report = run_integration_eval()
    if report["status"] == "skipped":
        pytest.skip(report["skip_reason"])

    assert report["status"] == "passed"
    assert report["threshold_failures"] == []

