#!/usr/bin/env python3
"""Pre-compute report data from persona metadata files.

Reads all {persona}.yaml files in a run directory, cross-references findings
and XREFs, and writes synthesis.yaml with pre-computed tables, aggregates,
corroboration matches, and disagreement detections.

Usage:
    python3 scripts/synthesize.py {run_dir} \\
        --source 3.3 --target 3.4 \\
        --personas admin,engineer,solution-architect,sre
"""

import argparse
import os
import sys
from datetime import datetime, timezone

import yaml

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from metadata import PERSONAS_CSV, ValidationError, read_metadata


class _LiteralStr(str):
    pass


def _literal_representer(dumper, data):
    return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|")


yaml.add_representer(_LiteralStr, _literal_representer)


SEVERITY_ORDER = ["BLOCKING", "HIGH", "MEDIUM", "LOW"]


def load_personas(run_dir, personas):
    loaded = {}
    missing = []
    inapplicable = []
    errors = {}
    for p in personas:
        path = os.path.join(run_dir, f"{p}.yaml")
        if not os.path.exists(path):
            missing.append(p)
            continue
        try:
            data = read_metadata(path)
            if data.get("inapplicable", False):
                inapplicable.append(p)
            else:
                loaded[p] = data
        except (ValidationError, yaml.YAMLError) as e:
            errors[p] = str(e)
    return loaded, missing, inapplicable, errors


def build_verdict_table(loaded, missing, inapplicable):
    rows = []
    for persona, data in loaded.items():
        top_concerns = [
            f["title"]
            for f in data["findings"]
            if f["severity"] in ("BLOCKING", "HIGH")
        ]
        concern_str = "; ".join(top_concerns) if top_concerns else "\u2014"
        rows.append(
            f"| {persona} | {data['risk_level']} | {concern_str} "
            f"| {data['recommendation']} |"
        )
    for p in inapplicable:
        rows.append(
            f"| {p} | N/A | domain not applicable to this transition | \u2014 |"
        )
    for p in missing:
        rows.append(
            f"| {p} | unknown | persona did not produce output | re-run |"
        )

    header = (
        "| Persona | Risk | Key Concerns | Recommendation |\n"
        "|---------|------|--------------|----------------|"
    )
    return header + "\n" + "\n".join(rows)


def build_persona_table(loaded, missing, inapplicable):
    rows = []
    for persona, data in loaded.items():
        counts = _finding_counts(data["findings"])
        count_str = " ".join(
            f"{counts[s]}{s[0]}" for s in SEVERITY_ORDER if counts[s] > 0
        )
        if not count_str:
            count_str = "0"
        rows.append(
            f"| {persona} | [{persona}.md](./{persona}.md) "
            f"| {count_str} | {data['resources_assessed']} |"
        )
    for p in inapplicable:
        rows.append(
            f"| {p} | [{p}.md](./{p}.md) | N/A | N/A |"
        )
    for p in missing:
        rows.append(
            f"| {p} | \u2014 | \u2014 | \u2014 |"
        )

    header = (
        "| Persona | Output | Findings | Resources |\n"
        "|---------|--------|----------|-----------|"
    )
    return header + "\n" + "\n".join(rows)


def _finding_counts(findings):
    counts = {s: 0 for s in SEVERITY_ORDER}
    for f in findings:
        sev = f["severity"]
        if sev in counts:
            counts[sev] += 1
    return counts


def compute_aggregate(loaded):
    agg = {s.lower(): 0 for s in SEVERITY_ORDER}
    agg["unverified"] = 0
    for data in loaded.values():
        for f in data["findings"]:
            key = f["severity"].lower()
            if key in agg:
                agg[key] += 1
        agg["unverified"] += data.get("unverified_claims", 0)
    return agg


def collect_blocking_findings(loaded):
    result = []
    for persona, data in loaded.items():
        for f in data["findings"]:
            if f["severity"] == "BLOCKING":
                result.append({"persona": persona, "title": f["title"]})
    return result


def find_corroborations(loaded):
    results = []
    for persona, data in loaded.items():
        for xref in data.get("xrefs", []):
            owner = xref["owner"]
            if owner not in loaded:
                continue
            owner_findings = loaded[owner]["findings"]
            match = _match_xref_to_finding(xref, owner_findings)
            if match:
                results.append({
                    "xref_persona": persona,
                    "xref_concern": xref["concern"],
                    "owner_persona": owner,
                    "owner_finding": match["title"],
                })
    return results


def find_disagreements(loaded):
    results = []
    for persona, data in loaded.items():
        for xref in data.get("xrefs", []):
            owner = xref["owner"]
            if owner not in loaded:
                continue
            owner_findings = loaded[owner]["findings"]
            match = _match_xref_to_finding(xref, owner_findings)
            if match and match["severity"] != xref["severity_hint"]:
                results.append({
                    "topic": xref["topic"],
                    "persona_a": persona,
                    "severity_a": xref["severity_hint"],
                    "persona_b": owner,
                    "severity_b": match["severity"],
                })
    return results


def _match_xref_to_finding(xref, findings):
    owner_id = xref.get("owner_finding_id")
    if owner_id:
        for f in findings:
            if f.get("finding_id") == owner_id:
                return f

    topic_words = set(xref["topic"].lower().split())
    best_match = None
    best_score = 0
    for f in findings:
        title_words = set(f["title"].lower().split())
        overlap = len(topic_words & title_words)
        if overlap > best_score:
            best_score = overlap
            best_match = f
    if best_score >= max(1, len(topic_words) // 2):
        return best_match
    return None


def synthesize(run_dir, source, target, personas, strict=False):
    loaded, missing, inapplicable, errors = load_personas(run_dir, personas)

    if errors:
        for p, err in errors.items():
            print(f"warning: {p}.yaml: {err}", file=sys.stderr)
        if strict:
            print("error: --strict mode: aborting due to metadata errors", file=sys.stderr)
            sys.exit(1)

    result = {
        "source": source,
        "target": target,
        "date": datetime.now(timezone.utc).strftime("%Y-%m-%d"),
        "verdict_table": _LiteralStr(build_verdict_table(loaded, missing, inapplicable)),
        "persona_table": _LiteralStr(build_persona_table(loaded, missing, inapplicable)),
        "aggregate": compute_aggregate(loaded),
        "blocking_findings": collect_blocking_findings(loaded),
        "corroborations": find_corroborations(loaded),
        "disagreements": find_disagreements(loaded),
        "missing_personas": missing,
        "inapplicable_personas": inapplicable,
    }

    if errors:
        result["errors"] = errors

    out_path = os.path.join(run_dir, "synthesis.yaml")
    with open(out_path, "w") as f:
        yaml.dump(
            result,
            f,
            default_flow_style=False,
            sort_keys=False,
            allow_unicode=True,
            width=120,
        )
    print(f"wrote {out_path}")
    return result


def main():
    parser = argparse.ArgumentParser(
        description="Pre-compute report data from persona metadata.")
    parser.add_argument("run_dir", help="Run directory containing persona .yaml files")
    parser.add_argument("--source", required=True, help="Source RHOAI version")
    parser.add_argument("--target", required=True, help="Target RHOAI version")
    parser.add_argument(
        "--personas",
        default=PERSONAS_CSV,
        help="Comma-separated persona list (default: all four)",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Fail if any persona metadata is invalid",
    )
    args = parser.parse_args()

    personas = [p.strip() for p in args.personas.split(",")]
    synthesize(args.run_dir, args.source, args.target, personas, strict=args.strict)


if __name__ == "__main__":
    main()
