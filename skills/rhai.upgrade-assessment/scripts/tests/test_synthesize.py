import os
import sys

import pytest
import yaml

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from metadata import write_metadata
from synthesize import (
    _match_xref_to_finding,
    build_verdict_table,
    find_corroborations,
    find_disagreements,
    load_personas,
    synthesize,
)


def _persona_data(persona, **overrides):
    base = {
        "persona": persona,
        "risk_level": "HIGH",
        "recommendation": "proceed-with-caution",
        "resources_assessed": 10,
        "findings": [
            {"severity": "HIGH", "title": "test finding", "confidence": "high",
             "finding_id": "test-finding"},
        ],
        "xrefs": [],
        "unverified_claims": 0,
        "runtime_checks": 2,
    }
    base.update(overrides)
    return base


def _write_persona(tmp_path, persona, **overrides):
    data = _persona_data(persona, **overrides)
    write_metadata(str(tmp_path / f"{persona}.yaml"), data)
    return data


class TestIdBasedMatching:
    def test_exact_id_match(self):
        xref = {"topic": "something unrelated", "owner": "sre",
                "concern": "x", "severity_hint": "HIGH",
                "owner_finding_id": "pod-restart"}
        findings = [
            {"severity": "HIGH", "title": "Pod restart during upgrade",
             "confidence": "high", "finding_id": "pod-restart"},
            {"severity": "MEDIUM", "title": "Other issue",
             "confidence": "medium", "finding_id": "other"},
        ]
        match = _match_xref_to_finding(xref, findings)
        assert match is not None
        assert match["finding_id"] == "pod-restart"

    def test_id_match_overrides_fuzzy(self):
        xref = {"topic": "Pod restart during upgrade", "owner": "sre",
                "concern": "x", "severity_hint": "HIGH",
                "owner_finding_id": "other"}
        findings = [
            {"severity": "HIGH", "title": "Pod restart during upgrade",
             "confidence": "high", "finding_id": "pod-restart"},
            {"severity": "MEDIUM", "title": "Other issue",
             "confidence": "medium", "finding_id": "other"},
        ]
        match = _match_xref_to_finding(xref, findings)
        assert match["finding_id"] == "other"


class TestFuzzyFallback:
    def test_matches_on_word_overlap(self):
        xref = {"topic": "pod restart disruption", "owner": "sre",
                "concern": "x", "severity_hint": "HIGH"}
        findings = [
            {"severity": "HIGH", "title": "Pod restart during upgrade",
             "confidence": "high"},
        ]
        match = _match_xref_to_finding(xref, findings)
        assert match is not None
        assert match["title"] == "Pod restart during upgrade"

    def test_returns_none_on_low_overlap(self):
        xref = {"topic": "completely different subject matter", "owner": "sre",
                "concern": "x", "severity_hint": "HIGH"}
        findings = [
            {"severity": "HIGH", "title": "Pod restart during upgrade",
             "confidence": "high"},
        ]
        match = _match_xref_to_finding(xref, findings)
        assert match is None


class TestCorroboration:
    def test_detects_corroboration(self, tmp_path):
        _write_persona(tmp_path, "sre", findings=[
            {"severity": "HIGH", "title": "Pod restart risk",
             "confidence": "high", "finding_id": "pod-restart"},
        ])
        _write_persona(tmp_path, "engineer", findings=[
            {"severity": "MEDIUM", "title": "CRD migration",
             "confidence": "high", "finding_id": "crd-migration"},
        ], xrefs=[
            {"topic": "pod restart", "owner": "sre",
             "concern": "API change extends window", "severity_hint": "HIGH",
             "owner_finding_id": "pod-restart"},
        ])
        loaded, _, _, _ = load_personas(str(tmp_path), ["sre", "engineer"])
        corrs = find_corroborations(loaded)
        assert len(corrs) == 1
        assert corrs[0]["xref_persona"] == "engineer"
        assert corrs[0]["owner_persona"] == "sre"


class TestDisagreement:
    def test_detects_severity_mismatch(self, tmp_path):
        _write_persona(tmp_path, "sre", findings=[
            {"severity": "MEDIUM", "title": "Gateway API migration",
             "confidence": "high", "finding_id": "gateway-api"},
        ])
        _write_persona(tmp_path, "architect", findings=[], xrefs=[
            {"topic": "gateway API", "owner": "sre",
             "concern": "breaking change", "severity_hint": "HIGH",
             "owner_finding_id": "gateway-api"},
        ])
        loaded, _, _, _ = load_personas(str(tmp_path), ["sre", "architect"])
        disagreements = find_disagreements(loaded)
        assert len(disagreements) == 1
        assert disagreements[0]["severity_a"] == "HIGH"
        assert disagreements[0]["severity_b"] == "MEDIUM"


class TestInapplicablePersona:
    def test_appears_in_inapplicable_list(self, tmp_path):
        write_metadata(
            str(tmp_path / "admin.yaml"),
            {"persona": "admin", "inapplicable": True},
        )
        _write_persona(tmp_path, "engineer")
        loaded, missing, inapplicable, errors = load_personas(
            str(tmp_path), ["engineer", "admin"]
        )
        assert "admin" in inapplicable
        assert "admin" not in loaded
        assert "admin" not in missing


class TestMissingPersona:
    def test_appears_in_missing_list(self, tmp_path):
        _write_persona(tmp_path, "engineer")
        loaded, missing, _, _ = load_personas(
            str(tmp_path), ["engineer", "sre"]
        )
        assert "sre" in missing
        assert "sre" not in loaded


class TestStrictMode:
    def test_strict_aborts_on_invalid_metadata(self, tmp_path):
        (tmp_path / "engineer.yaml").write_text("persona: engineer\nrisk_level: INVALID\n")
        with pytest.raises(SystemExit):
            synthesize(str(tmp_path), "3.3", "3.4", ["engineer"], strict=True)

    def test_non_strict_continues_on_invalid_metadata(self, tmp_path):
        (tmp_path / "engineer.yaml").write_text("persona: engineer\nrisk_level: INVALID\n")
        result = synthesize(str(tmp_path), "3.3", "3.4", ["engineer"], strict=False)
        assert "engineer" in result.get("errors", {})


class TestVerdictTable:
    def test_has_correct_headers(self):
        loaded = {"engineer": _persona_data("engineer")}
        table = build_verdict_table(loaded, [], [])
        assert "| Persona | Risk | Key Concerns | Recommendation |" in table
        assert "| engineer |" in table


class TestSynthesizeEndToEnd:
    def test_writes_synthesis_yaml(self, tmp_path):
        _write_persona(tmp_path, "engineer")
        _write_persona(tmp_path, "sre")
        result = synthesize(str(tmp_path), "3.3", "3.4", ["engineer", "sre"])
        synthesis_path = tmp_path / "synthesis.yaml"
        assert synthesis_path.exists()
        with open(synthesis_path) as f:
            data = yaml.safe_load(f)
        assert data["source"] == "3.3"
        assert data["target"] == "3.4"
        assert data["aggregate"]["high"] == 2
        assert data["missing_personas"] == []
