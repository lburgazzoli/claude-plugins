import os
import sys

import pytest
import yaml

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from metadata import ValidationError, read_metadata, validate, write_metadata


def _valid_data(**overrides):
    base = {
        "persona": "engineer",
        "risk_level": "HIGH",
        "recommendation": "proceed-with-caution",
        "resources_assessed": 10,
        "findings": [
            {"severity": "HIGH", "title": "test finding", "confidence": "high"},
        ],
        "xrefs": [],
        "unverified_claims": 0,
        "runtime_checks": 2,
    }
    base.update(overrides)
    return base


class TestWriteReadRoundtrip:
    def test_roundtrip(self, tmp_path):
        path = str(tmp_path / "engineer.yaml")
        data = _valid_data()
        write_metadata(path, data)
        result = read_metadata(path)
        assert result == data

    def test_roundtrip_with_finding_id(self, tmp_path):
        path = str(tmp_path / "engineer.yaml")
        data = _valid_data(findings=[
            {"severity": "HIGH", "title": "test", "confidence": "high",
             "finding_id": "test-finding"},
        ])
        write_metadata(path, data)
        result = read_metadata(path)
        assert result["findings"][0]["finding_id"] == "test-finding"

    def test_roundtrip_with_owner_finding_id(self, tmp_path):
        path = str(tmp_path / "engineer.yaml")
        data = _valid_data(xrefs=[
            {"topic": "restart", "owner": "sre", "concern": "window",
             "severity_hint": "HIGH", "owner_finding_id": "pod-restart"},
        ])
        write_metadata(path, data)
        result = read_metadata(path)
        assert result["xrefs"][0]["owner_finding_id"] == "pod-restart"


class TestValidation:
    def test_rejects_invalid_severity(self):
        data = _valid_data(findings=[
            {"severity": "CRITICAL", "title": "bad", "confidence": "high"},
        ])
        with pytest.raises(ValidationError, match="not in"):
            validate(data)

    def test_rejects_invalid_risk_level(self):
        data = _valid_data(risk_level="EXTREME")
        with pytest.raises(ValidationError, match="not in"):
            validate(data)

    def test_rejects_missing_required(self):
        data = _valid_data()
        del data["risk_level"]
        with pytest.raises(ValidationError, match="required field"):
            validate(data)

    def test_rejects_unknown_field(self):
        data = _valid_data(extra_field="nope")
        with pytest.raises(ValidationError, match="unknown field"):
            validate(data)

    def test_rejects_unknown_finding_field(self):
        data = _valid_data(findings=[
            {"severity": "HIGH", "title": "t", "confidence": "high", "bogus": 1},
        ])
        with pytest.raises(ValidationError, match="unknown field"):
            validate(data)

    def test_finding_id_optional(self):
        with_id = _valid_data(findings=[
            {"severity": "HIGH", "title": "t", "confidence": "high",
             "finding_id": "slug"},
        ])
        validate(with_id)

        without_id = _valid_data(findings=[
            {"severity": "HIGH", "title": "t", "confidence": "high"},
        ])
        validate(without_id)


class TestInapplicable:
    def test_skips_required_fields(self):
        data = {"persona": "admin", "inapplicable": True}
        validate(data)

    def test_still_requires_persona(self):
        data = {"inapplicable": True}
        with pytest.raises(ValidationError, match="persona"):
            validate(data)


class TestUnverifiedScan:
    def test_reports_warnings_on_invalid_yaml(self, tmp_path, capsys):
        (tmp_path / "bad.yaml").write_text("not: valid: yaml: {{")
        from metadata import cmd_unverified

        class Args:
            run_dir = str(tmp_path)

        cmd_unverified(Args())
        captured = capsys.readouterr()
        assert "warning: bad.yaml" in captured.err
        assert captured.out.strip() == "none"

    def test_reports_warnings_on_validation_error(self, tmp_path, capsys):
        (tmp_path / "bad.yaml").write_text(
            yaml.dump({"persona": "engineer", "risk_level": "INVALID"})
        )
        from metadata import cmd_unverified

        class Args:
            run_dir = str(tmp_path)

        cmd_unverified(Args())
        captured = capsys.readouterr()
        assert "warning: bad.yaml" in captured.err

    def test_skips_non_persona_files(self, tmp_path, capsys):
        for name in ("synthesis.yaml", "discrepancies.yaml", "state.yaml"):
            (tmp_path / name).write_text("not_persona: true")
        data = _valid_data(unverified_claims=1)
        path = str(tmp_path / "engineer.yaml")
        write_metadata(path, data)

        from metadata import cmd_unverified

        class Args:
            run_dir = str(tmp_path)

        cmd_unverified(Args())
        captured = capsys.readouterr()
        assert "engineer:1" in captured.out
        assert "synthesis" not in captured.out
        assert "discrepancies" not in captured.out
        assert "state" not in captured.out

    def test_reports_none_when_no_unverified(self, tmp_path, capsys):
        data = _valid_data(unverified_claims=0)
        write_metadata(str(tmp_path / "engineer.yaml"), data)

        from metadata import cmd_unverified

        class Args:
            run_dir = str(tmp_path)

        cmd_unverified(Args())
        captured = capsys.readouterr()
        assert captured.out.strip() == "none"
