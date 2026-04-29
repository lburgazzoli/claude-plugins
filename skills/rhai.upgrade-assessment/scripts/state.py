#!/usr/bin/env python3
"""State persistence for the upgrade assessment orchestrator.

Persists run configuration to a YAML file so long-running assessments
survive context compression. The orchestrator writes state at each step
boundary and reads it back if context is lost.

Usage:
    python3 scripts/state.py init {run_dir} --source 3.3 --target 3.4 \
        --scope static --personas admin,engineer,solution-architect,sre
    python3 scripts/state.py set {run_dir} --step 4 --status personas_spawned
    python3 scripts/state.py read {run_dir}
"""

import argparse
import json
import os
import sys

import yaml

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from metadata import PERSONAS_CSV


STATE_FILE = "state.yaml"


def state_path(run_dir):
    return os.path.join(run_dir, STATE_FILE)


def read_state(run_dir):
    path = state_path(run_dir)
    if not os.path.exists(path):
        return None
    with open(path) as f:
        return yaml.safe_load(f)


def write_state(run_dir, data):
    path = state_path(run_dir)
    with open(path, "w") as f:
        yaml.dump(data, f, default_flow_style=False, sort_keys=False)


def cmd_init(args):
    data = {
        "source": args.source,
        "target": args.target,
        "scope": args.scope,
        "personas": [p.strip() for p in args.personas.split(",")],
        "step": args.step,
        "status": "initialized",
        "run_dir": os.path.abspath(args.run_dir),
    }
    write_state(args.run_dir, data)
    print(f"initialized {state_path(args.run_dir)}")


def cmd_set(args):
    data = read_state(args.run_dir)
    if data is None:
        print(f"error: no state file in {args.run_dir}", file=sys.stderr)
        sys.exit(1)
    if args.step is not None:
        data["step"] = args.step
    if args.status is not None:
        data["status"] = args.status
    write_state(args.run_dir, data)
    print(f"updated step={data['step']} status={data['status']}")


def cmd_read(args):
    data = read_state(args.run_dir)
    if data is None:
        print(f"error: no state file in {args.run_dir}", file=sys.stderr)
        sys.exit(1)
    print(json.dumps(data, indent=2))


def main():
    parser = argparse.ArgumentParser(
        description="Upgrade assessment state persistence.")
    sub = parser.add_subparsers(dest="command")
    sub.required = True

    ip = sub.add_parser("init", help="Initialize state for a new run")
    ip.add_argument("run_dir")
    ip.add_argument("--source", required=True)
    ip.add_argument("--target", required=True)
    ip.add_argument("--scope", default="static")
    ip.add_argument("--personas", default=PERSONAS_CSV)
    ip.add_argument("--step", type=int, default=0)

    sp = sub.add_parser("set", help="Update state fields")
    sp.add_argument("run_dir")
    sp.add_argument("--step", type=int)
    sp.add_argument("--status")

    rp = sub.add_parser("read", help="Read current state")
    rp.add_argument("run_dir")

    args = parser.parse_args()
    {
        "init": cmd_init,
        "set": cmd_set,
        "read": cmd_read,
    }[args.command](args)


if __name__ == "__main__":
    main()
