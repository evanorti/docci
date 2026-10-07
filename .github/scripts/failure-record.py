#!/usr/bin/env python3
"""Build the failure record that docs-run hands to docs-repair.

The record is the whole interface between the guard and the fixer. It is
written down rather than rediscovered so that the repair job reproduces the
failure the guard actually saw -- an intermittent page would otherwise send the
fixer chasing a different failure than the one reported -- and so there is
something to read when the repair itself misbehaves.

docci prints two failure shapes in its "=== Validation Errors ===" section:
  - checkpoint mismatch: "... does not match the expected output ..." followed
    by "Expected:" and "Actual output:" blocks
  - output-contains: 'output does not contain "..."' followed by "Actual output:"
Both are handled, and every failure is recorded in `failures` (first ten;
`failures_omitted` counts the rest). The top-level step/expected/actual repeat
failures[0]. docci colours its log
lines with ANSI escapes, which are stripped before parsing and from the stored log (so `log` is not
byte-identical to docci's output; the raw bytes live in the CI run).
"""

import argparse
import json
import re
import sys

# GitHub's job summary caps at 1 MiB. Under half of that leaves room for the
# diagnosis the agent writes alongside it.
LOG_LIMIT = 400_000
FIELD_LIMIT = 50_000
# A page with dozens of failures is comprehensively broken; ten is enough to see
# the pattern behind them.
MAX_FAILURES = 10

ANSI_RE = re.compile(r"\x1b\[[0-9;?]*[A-Za-z]")
SECTION_MARK = "=== Validation Errors ==="
FAIL_RE = re.compile(r"^❌ \S+?:\d+ \((.*)\): (.*)$", re.M)
CONTAINS_RE = re.compile(r'^output does not contain (".*")\s*$')
# A docci log line (e.g. INFO(16:16:59) ...) or the next failure ends a block.
BLOCK_END_RE = re.compile(r"^(?:(?:INFO|ERROR|WARN|DEBUG)\(|❌ )", re.M)
# Fallback for logs without the summary section.
STEP_RE = re.compile(r"block=\S+:\d+ \((.+)\)\s*$", re.M)


def clip(text):
    if len(text) <= FIELD_LIMIT:
        return text
    return text[:FIELD_LIMIT] + f"\n... {len(text) - FIELD_LIMIT} characters truncated ..."


def truncate(log):
    """Keep the head and tail of an oversized log, never exceeding LOG_LIMIT.

    The failure is usually at the end and the setup at the start; the middle is
    the part nobody reads. If the validation summary falls in the middle (trailing
    cleanup output can push it there) its start is kept as a third piece. Every
    cut is a pure function of the input, so two runs of the same failure
    produce the same record.
    """
    if len(log) <= LOG_LIMIT:
        return log, False

    reserve = 300  # room for the notices
    budget = LOG_LIMIT - reserve
    head = tail = budget // 2
    section = log.find(SECTION_MARK)

    def notice(n):
        return f"\n\n... {n} characters truncated from the middle ...\n\n"

    if section != -1 and head <= section < len(log) - tail:
        # The summary sits in the middle: keep a quarter of the budget for it.
        mid = budget // 4
        head = tail = (budget - mid) // 2
        end = len(log) - tail
        return (log[:head] + notice(section - head) + log[section:section + mid]
                + notice(end - section - mid) + log[end:], True)

    omitted = len(log) - head - tail
    return log[:head] + notice(omitted) + log[len(log) - tail:], True


def parse_block(step, message, block):
    """Build one failure entry from its message line and the text that follows."""
    actual = ""
    head = block
    if "Actual output:\n" in block:
        head, actual = block.split("Actual output:\n", 1)

    contains = CONTAINS_RE.match(message)
    if contains:
        expected = f"output contains {contains.group(1)}"
    elif "\nExpected:\n" in "\n" + head:
        expected = ("\n" + head).split("\nExpected:\n", 1)[1]
    else:
        expected = ""
    return {"step": step, "expected": clip(expected.strip()), "actual": clip(actual.strip())}


def parse_failures(log):
    """Return every failure in the validation summary, in log order."""
    start = log.find(SECTION_MARK)
    if start == -1:
        found = STEP_RE.search(log)
        if not found:
            return []
        return [parse_block(found.group(1), "", log[found.end():])]

    body = log[start + len(SECTION_MARK):]
    matches = list(FAIL_RE.finditer(body))
    failures = []
    for i, fail in enumerate(matches):
        limit = matches[i + 1].start() if i + 1 < len(matches) else len(body)
        rest = body[fail.end():limit]
        end = BLOCK_END_RE.search(rest)
        block = rest[:end.start()] if end else rest
        failures.append(parse_block(fail.group(1), fail.group(2), block))
    return failures


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--page", required=True)
    parser.add_argument("--base-commit", required=True)
    parser.add_argument("--trigger", required=True, choices=["pull_request", "schedule"])
    parser.add_argument("--diff", default="")
    parser.add_argument("--upstream-range", default="")
    args = parser.parse_args()

    log = ANSI_RE.sub("", sys.stdin.read())
    truncated_log, was_truncated = truncate(log)
    all_failures = parse_failures(log)
    failures = all_failures[:MAX_FAILURES]
    first = failures[0] if failures else {"step": "", "expected": "", "actual": ""}

    record = {
        "page": args.page,
        # step/expected/actual mirror failures[0] for single-failure consumers.
        "step": first["step"],
        "expected": first["expected"],
        "actual": first["actual"],
        "failures": failures,
        "failures_omitted": len(all_failures) - len(failures),
        "log": truncated_log,
        "log_truncated": was_truncated,
        "base_commit": args.base_commit,
        "trigger": args.trigger,
        "diff": args.diff,
        "upstream_range": args.upstream_range,
    }

    json.dump(record, sys.stdout, indent=2)


if __name__ == "__main__":
    main()
