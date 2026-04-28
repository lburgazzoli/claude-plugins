#!/usr/bin/env python3
"""List and navigate upgrade assessment steps.

Reads step files from resources/steps/, parses YAML frontmatter,
and provides ordering and navigation based on scope and current state.

Usage:
    python3 scripts/steps.py list
    python3 scripts/steps.py list --scope static
    python3 scripts/steps.py show 3
    python3 scripts/steps.py next {run_dir}
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
    scope = state.get("scope", "static")
    steps = load_steps(scope=scope)

    for s in steps:
        if s["step"] > current_step:
            print(f"{s['step']} {s['file']}")
            return

    print("done")


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

    args = parser.parse_args()
    {"list": cmd_list, "show": cmd_show, "next": cmd_next}[args.command](args)


if __name__ == "__main__":
    main()
