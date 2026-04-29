#!/usr/bin/env python3
"""List and navigate upgrade assessment steps.

Reads step files from resources/steps/, parses YAML frontmatter,
and provides ordering and navigation based on scope and current state.

Usage:
    python3 scripts/steps.py list
    python3 scripts/steps.py list --scope static
    python3 scripts/steps.py show 3
    python3 scripts/steps.py next {run_dir}
    python3 scripts/steps.py next {run_dir} --flags dry-run
"""

import argparse
import glob
import os
import re
import sys

import yaml

STEPS_DIR = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
    "resources", "steps",
)


def parse_step_file(path):
    with open(path) as f:
        content = f.read()
    match = re.match(r"^---\n(.+?)\n---\n", content, re.DOTALL)
    if not match:
        return None
    meta = yaml.safe_load(match.group(1))
    basename = os.path.basename(path)
    num_match = re.match(r"(\d+)", basename)
    if not num_match:
        return None
    meta["step"] = int(num_match.group(1))
    meta["file"] = os.path.relpath(path, os.path.dirname(STEPS_DIR.rstrip("/")))
    meta["path"] = path
    return meta


def load_steps(scope=None):
    pattern = os.path.join(STEPS_DIR, "*.md")
    steps = []
    for path in sorted(glob.glob(pattern)):
        meta = parse_step_file(path)
        if meta is None:
            continue
        if scope and scope not in meta.get("scope", []):
            continue
        steps.append(meta)
    steps.sort(key=lambda s: s["step"])
    return steps


def expand_requires(requires, personas):
    """Expand {persona} templates in requires list."""
    expanded = []
    for req in requires:
        if "{persona}" in req:
            for p in personas:
                expanded.append(req.replace("{persona}", p))
        else:
            expanded.append(req)
    return expanded


def validate_artifacts(step_meta, run_dir, personas):
    """Check that all required artifacts exist. Returns list of missing files."""
    requires = step_meta.get("requires", [])
    if not requires:
        return []
    expanded = expand_requires(requires, personas)
    missing = []
    for artifact in expanded:
        path = os.path.join(run_dir, artifact)
        if not os.path.exists(path):
            missing.append(artifact)
    return missing


def cmd_list(args):
    steps = load_steps(scope=args.scope)
    if not steps:
        print("no steps found")
        return
    print(f"{'Step':<6} {'Name':<30} {'Scope':<20} {'Status'}")
    print(f"{'----':<6} {'----':<30} {'-----':<20} {'------'}")
    for s in steps:
        scope_str = ", ".join(s.get("scope", []))
        status = s.get("state-status", "—")
        print(f"{s['step']:<6} {s['name']:<30} {scope_str:<20} {status}")


def cmd_show(args):
    steps = load_steps()
    for s in steps:
        if s["step"] == args.step_number:
            with open(s["path"]) as f:
                print(f.read())
            return
    print(f"error: step {args.step_number} not found", file=sys.stderr)
    sys.exit(1)


def cmd_next(args):
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    from state import read_state

    state = read_state(args.run_dir)
    if state is None:
        print("error: no state file found", file=sys.stderr)
        sys.exit(1)

    current_step = state.get("step", 0)
    current_status = state.get("status", "")
    scope = state.get("scope", "static")
    personas = state.get("personas", [])
    run_dir = state.get("run_dir", args.run_dir)

    if current_status == "stopped":
        print("done")
        return

    flags = set()
    if args.flags:
        flags = {f.strip() for f in args.flags.split(",") if f.strip()}

    all_steps = load_steps(scope=scope)

    # Check if the just-completed step has a stop condition that matches
    if current_status != "running" and current_step > 0:
        for s in all_steps:
            if s["step"] == current_step and s.get("stop"):
                condition = s.get("stop-condition", "unconditional")
                if condition == "unconditional" or condition in flags:
                    print("done")
                    return
                break

    # Find the next step
    candidate = None
    for s in all_steps:
        if s["step"] > current_step:
            candidate = s
            break
        if s["step"] == current_step and current_status == "running":
            candidate = s
            break

    if candidate is None:
        print("done")
        return

    # Validate required artifacts before emitting the step
    missing = validate_artifacts(candidate, run_dir, personas)
    if missing:
        for m in missing:
            print(
                f"error: step {candidate['step']} requires {m} but it is missing",
                file=sys.stderr,
            )
        sys.exit(1)

    print(f"{candidate['step']} {candidate['file']}")


def main():
    parser = argparse.ArgumentParser(
        description="List and navigate upgrade assessment steps.")
    sub = parser.add_subparsers(dest="command")
    sub.required = True

    lp = sub.add_parser("list", help="List all steps")
    lp.add_argument("--scope", choices=["static", "runtime"],
                     help="Filter by scope")

    sp = sub.add_parser("show", help="Show step content")
    sp.add_argument("step_number", type=int, help="Step number to show")

    np = sub.add_parser("next", help="Get next step based on current state")
    np.add_argument("run_dir", help="Run directory containing state.yaml")
    np.add_argument("--flags", default="",
                    help="Comma-separated run flags (e.g., dry-run)")

    args = parser.parse_args()
    {"list": cmd_list, "show": cmd_show, "next": cmd_next}[args.command](args)


if __name__ == "__main__":
    main()
