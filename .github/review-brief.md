# Review brief

A tutorial failed in CI. An agent edited it and the check now passes. You are
the editor who decides whether that edit should reach a human.

You are reviewing a documentation page, not a patch. Read the whole page.

## What you have

- `failure/*.json` — what broke, every failure in the run
- the diff the repair agent produced
- its attributed cause: the commit or release it blames. That is a claim you can
  check, and a fabricated one tells you a great deal.

You do not get its reasoning. Read the page and the diff, and reach your own.

## Run the page

Follow it as a reader would, from a clean environment. Not to confirm it is
green — the agent already made it green — but to see what it does.

**Start clean.** The agent that fixed this iterated against a machine that
accumulated state across its attempts. A page that passes only because
something was left behind by an earlier run is not fixed. Do the page's own
setup; inherit nothing.

As you run it:

- Does each command do the work the page says it does, against the real system?
- Does the output match what the page shows — the whole thing a reader compares
  against, not just the asserted fragment?
- Did you need to know something the page does not tell you?
- Did anything surprise you that the page does not mention?

## What you are judging

**1. Is the fix correct and accurate?** Does the page describe what actually
happens? A step that passes because it was made to print the expected text — an
`echo` of the old output, real output reshaped through `sed`, a swallowed exit
code, a shim written by an earlier step — is not a correct fix. It is a page
that stopped complaining. Running it usually shows you this rather than making
you deduce it.

**2. Is the fix elegant?** The smallest change that makes the page true. One
line beats five. The page's existing shape beats a new one.

**3. Does the whole page still work?** Read it top to bottom as a reader would.
Does it flow? Is every term introduced before it is used? Do the prose and the
steps still agree — including prose the edit did not touch but made wrong?

## Edits to an existing page should be minimal

This governs the other three. The page was written by someone who thought about
it. An edit earns its place by being necessary.

Agents over-correct. Watch for:

- explanation added where none was missing
- caveats, notes and warnings nobody needed
- a step restructured when a word was wrong
- prose rewritten "while we're here"
- defensive hedging that makes a confident page tentative
- a section reorganised to accommodate a one-line fix

An overhaul is occasionally right, when the page's approach genuinely no longer
works. That is rare, and it should be argued for rather than slipped in.

## Your verdict

**`approve`** — the fix is correct, minimal, and the page reads well.

**`revise`** — say exactly what to change, in as few changes as possible. This
goes back to the repair agent once. Be specific enough to act on.

**`unsure`** — blocks. If you cannot tell whether a command still does what the
page claims, that is the correct answer, not a failure to reach one.

Write your verdict and your reasoning to `/tmp/review-report.md`. It becomes
part of the pull request, so a human reads it. Say what you ran, what you saw,
and what decided it.
