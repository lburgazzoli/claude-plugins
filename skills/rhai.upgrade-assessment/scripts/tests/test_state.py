import json
import os
import sys

import pytest
import yaml

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from state import cmd_init, cmd_read, cmd_set, read_state, state_path


class _Args:
    def __init__(self, **kwargs):
        for k, v in kwargs.items():
            setattr(self, k, v)


class TestInit:
    def test_creates_state_file(self, tmp_path):
        run_dir = str(tmp_path)
        cmd_init(_Args(
            run_dir=run_dir, source="3.3", target="3.4",
            scope="static", personas="sre,engineer", step=0,
        ))
        data = read_state(run_dir)
        assert data is not None
        assert data["source"] == "3.3"
        assert data["target"] == "3.4"
        assert data["scope"] == "static"
        assert data["personas"] == ["sre", "engineer"]
        assert data["step"] == 0
        assert data["status"] == "initialized"


class TestSet:
    def test_updates_step_and_status(self, tmp_path):
        run_dir = str(tmp_path)
        cmd_init(_Args(
            run_dir=run_dir, source="3.3", target="3.4",
            scope="static", personas="sre,engineer", step=0,
        ))
        cmd_set(_Args(run_dir=run_dir, step=4, status="personas_spawned"))
        data = read_state(run_dir)
        assert data["step"] == 4
        assert data["status"] == "personas_spawned"

    def test_missing_state_exits(self, tmp_path):
        with pytest.raises(SystemExit):
            cmd_set(_Args(run_dir=str(tmp_path), step=3, status="x"))


class TestRead:
    def test_outputs_json(self, tmp_path, capsys):
        run_dir = str(tmp_path)
        cmd_init(_Args(
            run_dir=run_dir, source="3.3", target="3.4",
            scope="static", personas="sre", step=0,
        ))
        capsys.readouterr()
        cmd_read(_Args(run_dir=run_dir))
        captured = capsys.readouterr()
        data = json.loads(captured.out)
        assert data["source"] == "3.3"

    def test_missing_state_exits(self, tmp_path):
        with pytest.raises(SystemExit):
            cmd_read(_Args(run_dir=str(tmp_path)))
