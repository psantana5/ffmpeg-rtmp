import json
from pathlib import Path

import run_tests


def test_parse_csv_list():
    assert run_tests.parse_csv_list("a,b, c") == ["a", "b", "c"]
    assert run_tests.parse_csv_list("  ") == []


def test_parse_bitrate_to_kbps():
    assert run_tests.parse_bitrate_to_kbps("1000k") == 1000
    assert run_tests.parse_bitrate_to_kbps("2.5M") == 2500
    assert run_tests.parse_bitrate_to_kbps("500") == 500


def test_load_batch_file_object(tmp_path: Path):
    p = tmp_path / "batch.json"
    p.write_text(json.dumps({"scenarios": [{"type": "baseline", "duration": 1}]}))
    scenarios = run_tests.load_batch_file(str(p))
    assert isinstance(scenarios, list)
    assert scenarios[0]["type"] == "baseline"


def test_load_batch_file_list(tmp_path: Path):
    p = tmp_path / "batch.json"
    p.write_text(json.dumps([{"type": "baseline", "duration": 1}]))
    scenarios = run_tests.load_batch_file(str(p))
    assert scenarios[0]["duration"] == 1
