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
    assert f[1] == {"kind": "validation", "step": "two", "expected": 'output contains "c1"', "actual": "a1"}
    assert f[0]["expected"] == "e0\nline2" and f[2]["actual"] == "a2\nmore"


def test_failure_list_is_capped_and_omitted_counted():
    record = build(_failure_section([("match", f"s{n}") for n in range(25)]))
    assert len(record["failures"]) == 10
    assert record["failures"][9]["step"] == "s9"
    assert record["failures_omitted"] == 15


# Real shape of a command failure (ANSI stripped by the script): no validation
# section, DEBUG-trap duplicates, docci bookkeeping commands.
COMMAND_FAIL = (
    "\x1b[34mINFO\x1b[0m(17:07:22) running docci file=/t/01.mdx\n"
    "     Executing CMD: go build -o toy ./toy-cli\n"
    "     Executing CMD: trap - DEBUG\n"
    "     Executing CMD: ./toy status\n"
    '{"state": "SUCCEEDED", "height": "12"}\n'
    "     Executing CMD: trap - DEBUG\n"
    "     Executing CMD: ./toy balance\n"
    'unknown command "balance"\n'
    "     Executing CMD: ./toy balance\n"
    "     Executing CMD: ./toy balance\n"
    "     Executing CMD: jobs -p\n"
    "     Executing CMD: xargs -r kill 2> /dev/null\n"
    "\x1b[31mERROR\x1b[0m(17:07:23) Unexpected script execution failure error=exit status 1\n"
    "\x1b[31mERROR\x1b[0m(17:07:23) Command failed exitCode=1\n"
)


def test_command_failure_is_recorded():
    record = build(COMMAND_FAIL)
    assert record["kind"] == "command"
    assert record["step"] == "./toy balance"  # not xargs/jobs/trap
    assert record["expected"] == "the command to exit 0"
    assert record["actual"] == 'unknown command "balance"'  # not the earlier ./toy status output
    assert record["failures"] == [{"kind": "command", "step": "./toy balance",
                                   "expected": "the command to exit 0",
                                   "actual": 'unknown command "balance"', "exit_code": 1}]


def test_command_failure_exit_status_is_read_from_the_log():
    record = build(COMMAND_FAIL.replace("exit status 1", "exit status 127"))
    assert record["failures"][0]["exit_code"] == 127


def test_repeated_command_is_one_occurrence_and_keeps_all_output():
    log = ("     Executing CMD: ./toy x\nfirst\n     Executing CMD: ./toy x\nsecond\n"
           "     Executing CMD: ./toy x\n"
           "ERROR(1:1:1) Unexpected script execution failure error=exit status 2\n")
    assert build(log)["actual"] == "first\nsecond"
    # a different command in between ends the run
    log2 = ("     Executing CMD: ./toy x\nold\n     Executing CMD: ./toy y\nnew\n"
            "     Executing CMD: ./toy x\n"
            "ERROR(1:1:1) Unexpected script execution failure error=exit status 2\n")
    r = build(log2)
    assert r["step"] == "./toy x" and r["actual"] == ""


def test_command_failure_plus_validation_failures():
    log = COMMAND_FAIL + _failure_section([("match", "one"), ("contains", "two")])
    record = build(log)
    assert [f["kind"] for f in record["failures"]] == ["command", "validation", "validation"]
    assert [f["step"] for f in record["failures"]] == ["./toy balance", "one", "two"]
    assert record["kind"] == "command" and record["step"] == "./toy balance"
    assert record["failures"][1]["expected"] == "e0\nline2"
    assert record["failures_omitted"] == 0


def test_validation_only_logs_have_no_command_failure():
    record = build(REAL)
    assert record["kind"] == "validation"
    assert [f["kind"] for f in record["failures"]] == ["validation"]
    # successful commands in the log must not be mistaken for a failure
    ok = build("     Executing CMD: ./toy a\nout\n")
    assert ok["failures"] == [] and ok["kind"] == ""
