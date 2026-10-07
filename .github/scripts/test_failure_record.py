import json
import subprocess
import sys
from pathlib import Path

SCRIPT = Path(__file__).parent / "failure-record.py"

# Captured from a real `docci run` (ANSI escapes included).
REAL = (
    "\x1b[34mINFO\x1b[0m(16:16:59) running docci file=/tmp/h.mdx\n"
    "  \"ready\": true\n"
    "\x1b[31mERROR\x1b[0m(16:16:59) Block output does not match its checkpoint block=/tmp/h.mdx:49 (counter reports a height)\n"
    "\x1b[31mERROR\x1b[0m(16:16:59) Found validation errors count=1\n\n"
    "=== Validation Errors ===\n"
    "❌ /tmp/h.mdx:49 (counter reports a height): output does not match the expected output shown on the page.\n"
    "first line that did not appear: \"ready\": false\n\n"
    "Expected:\n{\n  \"height\": \"<...>\",\n  \"ready\": false\n}\n\n\n"
    "Actual output:\n{\n  \"height\": \"3\",\n  \"ready\": true\n}\n"
    "\x1b[34mINFO\x1b[0m(16:16:59) Running command=rm -f counter.txt\n"
)

CONTAINS = (
    "\x1b[31mERROR\x1b[0m(16:17:08) Block output does not contain expected string block=/tmp/c.mdx:15 (under \"Before you start\") expected=zzz\n\n"
    "=== Validation Errors ===\n"
    "❌ /tmp/c.mdx:15 (under \"Before you start\"): output does not contain \"zzz\"\n"
    "Actual output:\nrunning under zshbash\n"
    "\x1b[34mINFO\x1b[0m(16:17:08) Cleanup complete\n"
)


def build(log_text, **kwargs):
    args = [sys.executable, str(SCRIPT), "--page", kwargs.get("page", "docs/x.mdx"),
            "--base-commit", "abc123", "--trigger", kwargs.get("trigger", "pull_request")]
    result = subprocess.run(args, input=log_text, capture_output=True, text=True, check=True)
    return json.loads(result.stdout)


def test_extracts_step_expected_and_actual():
    log = (
        'ERROR Block output does not match its checkpoint '
        'block=docs/x.mdx:49 (counter reports a height)\n'
        'first line that did not appear:   "ready": false\n'
        'Expected:\n  "ready": false\n'
        'Actual output:\n  "ready": true\n'
    )
    record = build(log)
    assert record["step"] == "counter reports a height"
    assert '"ready": false' in record["expected"]
    assert '"ready": true' in record["actual"]


def test_real_coloured_output_with_multiline_blocks():
    record = build(REAL)
    assert record["step"] == "counter reports a height"
    assert record["expected"] == '{\n  "height": "<...>",\n  "ready": false\n}'
    assert record["actual"] == '{\n  "height": "3",\n  "ready": true\n}'
    assert "\x1b" not in record["log"]


def test_output_contains_failure():
    record = build(CONTAINS)
    assert record["step"] == 'under "Before you start"'
    assert record["expected"] == 'output contains "zzz"'
    assert record["actual"] == "running under zshbash"


def test_no_failure_found_gives_empty_fields():
    record = build("all fine\n")
    assert record["step"] == record["expected"] == record["actual"] == ""


def test_truncates_from_the_middle_and_says_so():
    log = "\n".join(f"line {n}" for n in range(200000))
    record = build(log)
    assert record["log_truncated"] is True
    assert len(record["log"]) <= 400_000
    assert "line 0" in record["log"], "the start must survive"
    assert "line 199999" in record["log"], "the end must survive"
    assert "truncated" in record["log"]


def test_truncation_is_deterministic_and_keeps_the_failure_section():
    log = "noise\n" * 200000 + REAL + "trailing\n" * 200000
    first, second = build(log), build(log)
    assert first == second
    assert first["log_truncated"] is True
    assert len(first["log"]) <= 400_000
    assert "=== Validation Errors ===" in first["log"]
    assert first["actual"] == '{\n  "height": "3",\n  "ready": true\n}'


def _failure_section(entries):
    out = "=== Validation Errors ===\n"
    for n, (kind, step) in enumerate(entries):
        if kind == "match":
            out += (f"❌ /t.mdx:{n}0 ({step}): output does not match the expected output shown on the page.\n"
                    f"first line that did not appear: e{n}\n\nExpected:\ne{n}\nline2\n\n\n"
                    f"Actual output:\na{n}\nmore\n")
        else:
            out += (f"❌ /t.mdx:{n}0 ({step}): output does not contain \"c{n}\"\n"
                    f"Actual output:\na{n}\n")
    return out + "\x1b[34mINFO\x1b[0m(1:1:1) Cleanup complete\n"


def test_three_failures_each_recorded():
    record = build(_failure_section([("match", "one"), ("match", "two"), ("match", "three")]))
    assert [f["step"] for f in record["failures"]] == ["one", "two", "three"]
    assert [f["expected"] for f in record["failures"]] == ["e0\nline2", "e1\nline2", "e2\nline2"]
    assert [f["actual"] for f in record["failures"]] == ["a0\nmore", "a1\nmore", "a2\nmore"]
    assert record["failures_omitted"] == 0
    assert record["step"] == "one"


def test_mixed_failure_shapes():
    record = build(_failure_section([("match", "one"), ("contains", "two"), ("match", "three")]))
    f = record["failures"]
    assert f[1] == {"step": "two", "expected": 'output contains "c1"', "actual": "a1"}
    assert f[0]["expected"] == "e0\nline2" and f[2]["actual"] == "a2\nmore"


def test_failure_list_is_capped_and_omitted_counted():
    record = build(_failure_section([("match", f"s{n}") for n in range(25)]))
    assert len(record["failures"]) == 10
    assert record["failures"][9]["step"] == "s9"
    assert record["failures_omitted"] == 15
