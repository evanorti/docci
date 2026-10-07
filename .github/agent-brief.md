# Repair brief

A tutorial failed in CI. Your job is to make the page work for the person
following it, or to explain why you cannot. Both are acceptable outcomes.

**A green check is not the goal.** docci is the instrument that noticed this
page stopped being true. A page can always be made green by editing what it
claims the output is, and that leaves the reader with a tutorial that still does
not work. It has only stopped complaining. You are fixing the page a human
follows. The check turning green is evidence that you did.

## Fix at the tier the real change belongs to

The tiers describe what actually changed. They are not fallbacks to try in turn.

1. **The step is wrong.** The page tells the reader to run the wrong thing: a
   renamed flag, a changed argument, a step that now needs a different order.
   Best outcome. The tutorial was wrong for a human and now it is not.
2. **The prose is wrong.** The explanation no longer matches what happens.
3. **The expected output is wrong.** The tool genuinely prints something
   different. Legitimate, but it changes what the page claims, not what it does.
4. **The instrumentation is wrong.** A `retry`, a wider `<...>`, a longer
   timeout. Only when something genuinely varies between runs.

Work out which tier the change belongs to, and fix there. If a flag was renamed,
the step is wrong, and editing the output cannot help. If the tool now prints a
different shape, the output is wrong, and editing the step cannot help. Guessing
at the wrong tier spends one of your three attempts on an edit that was never
going to work.

The order is a preference for when more than one tier would genuinely work: take
the lower number. It is also a warning. Editing expected output is the shortest
path to green, so it is the fix you will reach for by reflex, and it is the one
that leaves a reader stuck. Before you settle on tier 3 or 4, ask what the
reader would have to do differently for the old output to be right. If there is
an answer, the fix belongs at tier 1 or 2.

Your report states which tier the fix landed in and why each higher tier did not
apply.

## What you have

`failure/*.json` is the failure record. It holds the page, the diff under test
(pull request) or the upstream range (schedule), the log, and a `failures` list.
That list is every failure in the run, first ten, with `failures_omitted`
counting the rest. Each entry has the step, the expected output and the actual
output. The top-level `step`, `expected` and `actual` only repeat `failures[0]`.

A live environment with docci installed and the page's dependencies up.

## What to do

1. **Read every failure before you edit anything.** Fixing them one at a time
   produces piecemeal edits, and failures in one run usually share a cause. If
   `failures_omitted` is above zero, treat the page as broadly broken and look
   harder at the cause before editing.

2. **Diagnose.** Find what changed. For a pull request, read the diff. For a
   schedule, read the upstream range named in the record. Name the commit or
   release you believe caused this. If you cannot find one, say so. That is
   evidence the code regressed rather than the docs drifting.

3. **Edit toward a page a reader can follow.** The page is a tutorial: one
   heading per imperative step, runnable commands with nothing to substitute by
   hand, output rendered only at real checkpoints. A fix keeps it that way.
   Directives go in comments above the block, never in the fence info string.
   Assert on a key and its value, not a bare word. The syntax is in docci's
   README.

4. **Run the whole page.** `docci run <page>`. Not a partial check. A fix that
   makes step 4 pass while breaking step 7 is not a fix.

5. **Check your own diff.** `docci review <base-page> <edited-page>`, where the
   base is the page as it was before you touched it.
   - Exit 0: the edit claims what the page claimed before. Proceed.
   - Exit 2: the edit claims less than it did in a way that matters. **Stop.**
   - Exit 1: the two pages could not be compared. **Stop.**

   The tool makes the judgement, so you do not have to. It classifies each edit.
   Only a reduction that removes something load-bearing, or replacing a derived
   value with a literal, blocks with 2. A reduction that removes noise
   (timestamps, progress lines, paths) is allowed and exits 0.

   On 2 or 1, do not try a different edit to get round it. Report.

   The same check runs again in the workflow after you finish, outside your
   control, and it decides whether a pull request is opened. You gain nothing by
   edging past it, and a human reads what you report in either case.

6. **Three attempts, then stop.** If the page is not green and clean by the
   third, report.

## What you may change

Commands, flags, step order, prose, and assertions, as long as `docci review`
exits 0.

## What you may not do

- Weaken an assertion to fit the failure. If the only way to pass is to claim
  less, the behaviour changed and a human decides whether that is correct.
- Replace a derived value (`$(...)`, `$VAR`) with a literal. The page then
  passes on one machine and breaks for every reader.
- Delete an assertion.
- Edit the product's source. You are fixing documentation.

## What to produce

Whatever the outcome, write your report to `/tmp/repair-report.md`. When a pull
request opens, that file is its body, so it states what changed, the attributed
cause, which tier the fix landed in and why no higher tier applied, and the
before and after of every assertion touched. When none opens, it is the report
a human reads instead.

Leave your edits in the working tree and do not commit them. The workflow
commits and opens the pull request, and the action collects the changes itself,
so a commit of your own leaves it with nothing to collect.

**Green and `docci review` exits 0:** a pull request, one for the page. The body
gives:
- the attributed cause,
- the tier the fix landed in, and why no higher tier applied,
- before and after for every assertion touched,
- for any reduction, one reason: *varies per run*, *no longer emitted upstream*,
  or *never load-bearing*.

**Green only by weakening:** no pull request. Report that the page passes only
if it claims less, which suggests the behaviour genuinely changed. Include the
diff you wanted to make, so a human can take it in one action if it is right.

**Not green after three attempts, or review exited 1:** no pull request. Report
all attempts and what each failed on.

Every report carries the page and step, expected versus actual, what you tried,
and your best attribution for the cause.

The pull request body is a plain description of the change.
No attribution footers, and no mention of Claude or any tool that wrote them.
