#!/usr/bin/env python3
"""CLI for writing, reading, and validating persona assessment metadata.

Personas call this script to write structured .yaml sidecar files alongside
their .md prose output. The synthesis script reads these files mechanically.

Usage:
    python3 scripts/metadata.py schema
    python3 scripts/metadata.py write {path} --persona sre --risk_level HIGH ...
    python3 scripts/metadata.py read {path}
    python3 scripts/metadata.py unverified {run_dir}
"""

import argparse
import glob
import json
import os
import sys

import yaml


def _discover_personas():
    personas_dir = os.path.join(
        os.path.dirname(__file__), os.pardir,
        "resources", "prompts", "personas",
    )
    personas_dir = os.path.normpath(personas_dir)
    return sorted(
        os.path.splitext(f)[0]
        for f in os.listdir(personas_dir)
        if f.endswith(".md")
    )


PERSONAS = _discover_personas()
PERSONAS_CSV = ",".join(PERSONAS)

SCHEMA = {
    "persona": {
        "type": "string",
        "required": True,
        "enum": PERSONAS,
    },
    "inapplicable": {
        "type": "bool",
        "required": False,
        "default": False,
    },
    "risk_level": {
        "type": "string",
        "required": True,
        "enum": ["BLOCKING", "HIGH", "MEDIUM", "LOW", "NONE"],
    },
    "recommendation": {
        "type": "string",
        "required": True,
        "enum": ["proceed", "proceed-with-caution", "delay", "block"],
    },
    "resources_assessed": {
        "type": "int",
        "required": True,
    },
    "findings": {
        "type": "list",
        "required": True,
        "item_fields": {
            "severity": {
                "type": "string",
                "required": True,
                "enum": ["BLOCKING", "HIGH", "MEDIUM", "LOW"],
            },
            "title": {
                "type": "string",
                "required": True,
            },
            "confidence": {
                "type": "string",
                "required": True,
                "enum": ["high", "medium", "low"],
            },
            "finding_id": {
                "type": "string",
                "required": False,
            },
        },
    },
    "xrefs": {
        "type": "list",
        "required": False,
        "default": [],
        "item_fields": {
            "topic": {
                "type": "string",
                "required": True,
            },
            "owner": {
                "type": "string",
                "required": True,
                "enum": PERSONAS,
            },
            "concern": {
                "type": "string",
                "required": True,
            },
            "severity_hint": {
                "type": "string",
                "required": True,
                "enum": ["HIGH", "MEDIUM", "LOW"],
            },
            "owner_finding_id": {
                "type": "string",
                "required": False,
            },
        },
    },
    "unverified_claims": {
        "type": "int",
        "required": True,
    },
    "runtime_checks": {
        "type": "int",
        "required": True,
    },
}


class ValidationError(Exception):
    pass


def validate_field(name, value, spec):
    field_type = spec["type"]
    if field_type == "string":
        if not isinstance(value, str):
            raise ValidationError(f"{name}: expected string, got {type(value).__name__}")
        if "enum" in spec and value not in spec["enum"]:
            raise ValidationError(
                f"{name}: '{value}' not in {spec['enum']}"
            )
    elif field_type == "int":
        if not isinstance(value, int) or isinstance(value, bool):
            raise ValidationError(f"{name}: expected int, got {type(value).__name__}")
    elif field_type == "bool":
        if not isinstance(value, bool):
            raise ValidationError(f"{name}: expected bool, got {type(value).__name__}")
    elif field_type == "list":
        if not isinstance(value, list):
            raise ValidationError(f"{name}: expected list, got {type(value).__name__}")
        item_fields = spec.get("item_fields", {})
        for i, item in enumerate(value):
            if not isinstance(item, dict):
                raise ValidationError(f"{name}[{i}]: expected dict, got {type(item).__name__}")
            for fname, fspec in item_fields.items():
                if fname not in item:
                    if fspec.get("required", False):
                        raise ValidationError(f"{name}[{i}].{fname}: required field missing")
                    continue
                validate_field(f"{name}[{i}].{fname}", item[fname], fspec)
            for key in item:
                if key not in item_fields:
                    raise ValidationError(f"{name}[{i}]: unknown field '{key}'")


INAPPLICABLE_OPTIONAL = {
    "risk_level", "recommendation", "resources_assessed",
    "findings", "xrefs", "unverified_claims", "runtime_checks",
}


def validate(data):
    is_inapplicable = data.get("inapplicable", False)
    for name, spec in SCHEMA.items():
        if name not in data:
            if spec.get("required", False):
                if is_inapplicable and name in INAPPLICABLE_OPTIONAL:
                    continue
                raise ValidationError(f"required field '{name}' missing")
            continue
        validate_field(name, data[name], spec)
    for key in data:
        if key not in SCHEMA:
            raise ValidationError(f"unknown field '{key}'")
    return data


def read_metadata(path):
    with open(path) as f:
        data = yaml.safe_load(f)
    if data is None:
        raise ValidationError(f"{path}: empty or invalid YAML")
    validate(data)
    return data


def write_metadata(path, data):
    validate(data)
    with open(path, "w") as f:
        yaml.dump(data, f, default_flow_style=False, sort_keys=False, allow_unicode=True)


def get_schema_yaml():
    lines = []
    for name, spec in SCHEMA.items():
        required = "required" if spec.get("required") else "optional"
        enum_str = f", values: {spec['enum']}" if "enum" in spec else ""
        lines.append(f"{name}: {spec['type']} ({required}{enum_str})")
        if spec["type"] == "list" and "item_fields" in spec:
            for fname, fspec in spec["item_fields"].items():
                freq = "required" if fspec.get("required") else "optional"
                fenum = f", values: {fspec['enum']}" if "enum" in fspec else ""
                lines.append(f"  {fname}: {fspec['type']} ({freq}{fenum})")
    return "\n".join(lines)


def cmd_schema(_args):
    print(get_schema_yaml())


def cmd_write(args):
    if args.inapplicable:
        data = {
            "persona": args.persona,
            "inapplicable": True,
        }
        try:
            write_metadata(args.path, data)
            print(f"wrote {args.path}")
        except ValidationError as e:
            print(f"validation error: {e}", file=sys.stderr)
            sys.exit(1)
        return
    required = ["risk_level", "recommendation", "resources_assessed",
                 "unverified_claims", "runtime_checks"]
    missing = [f for f in required if getattr(args, f) is None]
    if missing:
        print(f"error: required flags when not --inapplicable: --{'  --'.join(missing)}", file=sys.stderr)
        sys.exit(1)
    data = {
        "persona": args.persona,
        "risk_level": args.risk_level,
        "recommendation": args.recommendation,
        "resources_assessed": args.resources_assessed,
        "findings": [],
        "xrefs": [],
        "unverified_claims": args.unverified_claims,
        "runtime_checks": args.runtime_checks,
    }
    for f in (args.finding or []):
        if len(f) not in (3, 4):
            print(f"error: --finding requires 3-4 values (SEVERITY TITLE CONFIDENCE [FINDING_ID]), got {len(f)}", file=sys.stderr)
            sys.exit(1)
        entry = {"severity": f[0], "title": f[1], "confidence": f[2]}
        if len(f) == 4:
            entry["finding_id"] = f[3]
        data["findings"].append(entry)
    for x in (args.xref or []):
        if len(x) not in (4, 5):
            print(f"error: --xref requires 4-5 values (TOPIC OWNER CONCERN SEVERITY_HINT [OWNER_FINDING_ID]), got {len(x)}", file=sys.stderr)
            sys.exit(1)
        entry = {"topic": x[0], "owner": x[1], "concern": x[2], "severity_hint": x[3]}
        if len(x) == 5:
            entry["owner_finding_id"] = x[4]
        data["xrefs"].append(entry)
    try:
        write_metadata(args.path, data)
        print(f"wrote {args.path}")
    except ValidationError as e:
        print(f"validation error: {e}", file=sys.stderr)
        sys.exit(1)


def cmd_read(args):
    try:
        data = read_metadata(args.path)
        print(json.dumps(data, indent=2))
    except (ValidationError, FileNotFoundError) as e:
        print(f"error: {e}", file=sys.stderr)
        sys.exit(1)


def cmd_unverified(args):
    pattern = os.path.join(args.run_dir, "*.yaml")
    found = False
    for path in sorted(glob.glob(pattern)):
        basename = os.path.basename(path)
        if basename in ("synthesis.yaml", "discrepancies.yaml", "state.yaml"):
            continue
        try:
            data = read_metadata(path)
        except ValidationError as e:
            print(f"warning: {basename}: {e}", file=sys.stderr)
            continue
        except (yaml.YAMLError, OSError) as e:
            print(f"warning: {basename}: {e}", file=sys.stderr)
            continue
        count = data.get("unverified_claims", 0)
        if count > 0:
            print(f"{data['persona']}:{count}")
            found = True
    if not found:
        print("none")


def main():
    parser = argparse.ArgumentParser(
        description="Persona assessment metadata CLI.")
    sub = parser.add_subparsers(dest="command")
    sub.required = True

    sub.add_parser("schema", help="Show metadata schema")

    wp = sub.add_parser("write", help="Write validated metadata")
    wp.add_argument("path", help="Output .yaml file path")
    wp.add_argument("--persona", required=True)
    wp.add_argument("--inapplicable", action="store_true",
                    help="Mark persona as inapplicable (no other fields required)")
    wp.add_argument("--risk_level")
    wp.add_argument("--recommendation")
    wp.add_argument("--resources_assessed", type=int)
    wp.add_argument("--finding", nargs="+", action="append",
                    metavar="ARG",
                    help="Repeatable: --finding SEVERITY TITLE CONFIDENCE [FINDING_ID]")
    wp.add_argument("--xref", nargs="+", action="append",
                    metavar="ARG",
                    help="Repeatable: --xref TOPIC OWNER CONCERN SEVERITY_HINT [OWNER_FINDING_ID]")
    wp.add_argument("--unverified_claims", type=int)
    wp.add_argument("--runtime_checks", type=int)

    rp = sub.add_parser("read", help="Read and validate metadata as JSON")
    rp.add_argument("path", help=".yaml file to read")

    up = sub.add_parser("unverified", help="List personas with unverified claims")
    up.add_argument("run_dir", help="Run directory to scan")

    args = parser.parse_args()
    {
        "schema": cmd_schema,
        "write": cmd_write,
        "read": cmd_read,
        "unverified": cmd_unverified,
    }[args.command](args)


if __name__ == "__main__":
    main()
