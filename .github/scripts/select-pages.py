#!/usr/bin/env python3
"""Decide which docs pages a pull request needs to run.

Running every page on every pull request is slow enough that people turn the job
off. Running only the changed pages is wrong, because a page that others declare
as a prerequisite breaks them when it changes. This selects the changed pages
plus everything that depends on them, and drops pages that are not executable.

Usage:
    select-pages.py <changed-file> [<changed-file> ...]

Prints a JSON array of page paths, for a GitHub Actions matrix.
"""

import json
import os
import sys

import yaml

PAGE_EXTENSIONS = (".md", ".mdx")

# A page is executable if it carries at least one docci directive comment.
DIRECTIVE_MARKERS = ("{/* docci", "<!-- docci")


def read_frontmatter(path):
    """Return a page's `docci:` config, or an empty dict."""
    try:
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
    except (OSError, UnicodeDecodeError):
        return {}, ""

    if not text.startswith("---"):
        return {}, text

    parts = text.split("\n---", 1)
    if len(parts) < 2:
        return {}, text

    try:
        loaded = yaml.safe_load(parts[0].lstrip("-\n")) or {}
    except yaml.YAMLError:
        return {}, text

    config = loaded.get("docci") or {}
    return config if isinstance(config, dict) else {}, parts[1]


def all_pages(root="."):
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in {".git", "node_modules"}]
        for name in filenames:
            if name.endswith(PAGE_EXTENSIONS):
                yield os.path.relpath(os.path.join(dirpath, name), root)


def resolve_need(need, from_page):
    """Resolve a `needs` entry the way docci does."""
    if need.startswith("/"):
        base = need.lstrip("/")
    else:
        base = os.path.normpath(os.path.join(os.path.dirname(from_page), need))

    for extension in ("", ".mdx", ".md"):
        candidate = base + extension
        if os.path.isfile(candidate):
            return os.path.normpath(candidate)
    return None


def build_dependents():
    """Map each page to the pages that declare it as a prerequisite."""
    dependents = {}
    executable = set()

    for page in all_pages():
        config, body = read_frontmatter(page)

        if any(marker in body for marker in DIRECTIVE_MARKERS):
            executable.add(os.path.normpath(page))

        for need in config.get("needs") or []:
            resolved = resolve_need(need, page)
            if resolved:
                dependents.setdefault(resolved, set()).add(os.path.normpath(page))

    return dependents, executable


def expand(changed, dependents):
    """Add everything that transitively depends on a changed page."""
    affected = set()
    queue = list(changed)

    while queue:
        page = queue.pop()
        if page in affected:
            continue
        affected.add(page)
        queue.extend(dependents.get(page, ()))

    return affected


def main(argv):
    changed = {
        os.path.normpath(path)
        for path in argv
        if path.endswith(PAGE_EXTENSIONS) and os.path.isfile(path)
    }

    dependents, executable = build_dependents()
    affected = expand(changed, dependents)

    pages = []
    for page in sorted(affected):
        if page not in executable:
            continue

        config, _ = read_frontmatter(page)
        if config.get("runnable") is False:
            continue

        # A page with prerequisites is run by naming the page itself; docci
        # pulls its prerequisites in and runs them first.
        pages.append(page)

    print(json.dumps(pages))


if __name__ == "__main__":
    main(sys.argv[1:])
