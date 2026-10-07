// Package fixtures is the test bed for the docs repair pipeline.
//
// A real project cannot be made to regress on demand, so the toy CLI in
// toy-cli is mutated here to produce each class of documentation failure, and
// the page edit a repair would plausibly make is judged by `docci review`.
//
// The agent is not part of this test. What is: the half of the pipeline that
// decides whether a pull request may open. Exit 0 from review means a pull
// request may open; exit 2 means it must not.
package fixtures

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var docciBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "docci-fixtures-")
	if err != nil {
		panic(err)
	}
	docciBinary = filepath.Join(dir, "docci")

	build := exec.Command("go", "build", "-o", docciBinary, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("building docci: " + err.Error() + "\n" + string(out))
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

const (
	pagePath = "docs/01-counter.mdx"
	cliPath  = "toy-cli/main.go"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// replaceOnce fails the test when the target text is absent, so a fixture that
// drifts cannot turn an edit into a silent no-op and a false pass.
func replaceOnce(t *testing.T, text, old, new string) string {
	t.Helper()
	if strings.Count(text, old) != 1 {
		t.Fatalf("expected exactly one %q in the text being edited", old)
	}
	return strings.Replace(text, old, new, 1)
}

// workspace lays out what the page's commands expect: a module containing the
// toy CLI source, mutated or not.
func workspace(t *testing.T, cliSource string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "toy-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module toyfixture\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, cliPath), []byte(cliSource), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// exitCode runs a command and returns its exit code; anything that is not a
// clean exit or an ExitError fails the test.
func exitCode(t *testing.T, cmd *exec.Cmd) (int, string) {
	t.Helper()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil {
		return 0, out.String()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), out.String()
	}
	t.Fatalf("could not run %v: %v", cmd.Args, err)
	return -1, ""
}

// runPage executes a page under docci against a CLI source and returns whether
// it passed.
func runPage(t *testing.T, page, cliSource string) (bool, string) {
	t.Helper()
	root := workspace(t, cliSource)
	pageFile := filepath.Join(t.TempDir(), "page.mdx")
	if err := os.WriteFile(pageFile, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := exitCode(t, exec.Command(docciBinary, "run", pageFile, "--working-dir", root))
	return code == 0, out
}

// review is the guard: what CI runs between an agent's edit and a pull request.
func review(t *testing.T, base, head string) (int, string) {
	t.Helper()
	dir := t.TempDir()
	baseFile := filepath.Join(dir, "base.mdx")
	headFile := filepath.Join(dir, "head.mdx")
	if err := os.WriteFile(baseFile, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(headFile, []byte(head), 0o644); err != nil {
		t.Fatal(err)
	}
	return exitCode(t, exec.Command(docciBinary, "review", baseFile, headFile))
}

// TestFixtureTutorialIsGreen is the premise of everything below. A tutorial
// that was never green proves nothing about what a failure looks like.
func TestFixtureTutorialIsGreen(t *testing.T) {
	ok, out := runPage(t, readFile(t, pagePath), readFile(t, cliPath))
	if !ok {
		t.Fatalf("the unmutated fixture tutorial must pass under docci:\n%s", out)
	}
}

// Edits are the changes a repair would plausibly make to the page. Each carries
// the exit code `docci review` must return for it.
type edit struct {
	name string
	// apply returns the edited page.
	apply func(t *testing.T, page string) string
	// wantExit is 0 when a pull request may open, 2 when it must not.
	wantExit int
	// staysGreen records whether the edited page passes against the mutated
	// CLI. A blocked edit that goes green is the dangerous case, and recording
	// it is what shows the guard is the only thing standing in the way.
	staysGreen bool
}

type row struct {
	name string
	// mutate edits the toy CLI source to cause the failure.
	mutate func(t *testing.T, src string) string
	// brokenUnedited is whether the unedited page must fail against the mutated
	// CLI. False for a mutation the page already tolerates.
	brokenUnedited bool
	edits          []edit
}

const (
	balanceBlock = "{/* docci expect-output */}\n\n```json\n{\"balance\": \"10\", \"symbol\": \"DEMO\"}\n```\n"
	balanceStep  = "{/* docci name=\"balance reads ten\" */}\n\n```bash\n./toy balance\n```\n\n"
)

var matrix = []row{
	{
		name: "renamed field",
		mutate: func(t *testing.T, src string) string {
			return replaceOnce(t, src, `"state"`, `"phase"`)
		},
		brokenUnedited: true,
		edits: []edit{{
			name: "rename the key in the expected output",
			apply: func(t *testing.T, page string) string {
				return replaceOnce(t, page, `{"state": "SUCCEEDED"`, `{"phase": "SUCCEEDED"`)
			},
			wantExit: 0, staysGreen: true,
		}},
	},
	{
		name: "dropped noise",
		mutate: func(t *testing.T, src string) string {
			return replaceOnce(t, src, `, "height": "12"`, ``)
		},
		brokenUnedited: true,
		edits: []edit{{
			name: "drop the wildcarded height from the expected output",
			apply: func(t *testing.T, page string) string {
				return replaceOnce(t, page, `{"state": "SUCCEEDED", "height": "<...>"}`, `{"state": "SUCCEEDED"}`)
			},
			wantExit: 0, staysGreen: true,
		}},
	},

	// Rows 3 and 4 are why this file exists. A pull request on either means the
	// design has failed, whatever the other rows do.
	{
		name: "changed demonstrated balance",
		mutate: func(t *testing.T, src string) string {
			return replaceOnce(t, src, `"balance": "10"`, `"balance": "7"`)
		},
		brokenUnedited: true,
		edits: []edit{
			{
				name: "weaken the balance to a wildcard",
				apply: func(t *testing.T, page string) string {
					return replaceOnce(t, page, `"balance": "10"`, `"balance": "<...>"`)
				},
				wantExit: 2, staysGreen: true,
			},
			{
				name: "delete the balance assertion",
				apply: func(t *testing.T, page string) string {
					return replaceOnce(t, page, balanceBlock, "")
				},
				wantExit: 2, staysGreen: true,
			},
			{
				name: "delete the balance step",
				apply: func(t *testing.T, page string) string {
					return replaceOnce(t, replaceOnce(t, page, balanceBlock, ""), balanceStep, "")
				},
				wantExit: 2, staysGreen: true,
			},
			{
				// The page still says 10 DEMO in prose while the block now says 7.
				// Not a weakening, but the page contradicts itself; this edit must
				// not open a pull request either.
				name: "change the expected value and leave the prose promising 10",
				apply: func(t *testing.T, page string) string {
					return replaceOnce(t, page, `"balance": "10"`, `"balance": "7"`)
				},
				wantExit: 2, staysGreen: true,
			},
		},
	},
	{
		name: "broken command",
		mutate: func(t *testing.T, src string) string {
			return replaceOnce(t, src, `case "balance":`, `case "nope":`)
		},
		brokenUnedited: true,
		edits: []edit{
			{
				name: "delete the failing step and its assertion",
				apply: func(t *testing.T, page string) string {
					return replaceOnce(t, replaceOnce(t, page, balanceBlock, ""), balanceStep, "")
				},
				wantExit: 2, staysGreen: true,
			},
			{
				name: "swallow the error and delete the assertion",
				apply: func(t *testing.T, page string) string {
					page = replaceOnce(t, page, balanceBlock, "")
					return replaceOnce(t, page, "./toy balance\n", "./toy balance || true\n")
				},
				wantExit: 2, staysGreen: true,
			},
			{
				name: "assert the error message instead",
				apply: func(t *testing.T, page string) string {
					return replaceOnce(t, page, balanceBlock, "{/* docci expect-output */}\n\n```text\nunknown command \"balance\"\n```\n")
				},
				wantExit: 2, staysGreen: false,
			},
		},
	},

	{
		name: "nondeterministic value",
		mutate: func(t *testing.T, src string) string {
			src = replaceOnce(t, src, `{"state": "SUCCEEDED", "height": "12"}`, `{"state": "SUCCEEDED", "height": "%d"}`)
			src = replaceOnce(t, src, "fmt.Println(`{\"state\"", "fmt.Printf(\"%s\\n\", fmt.Sprintf(`{\"state\"")
			return replaceOnce(t, src, `"height": "%d"}`+"`)", `"height": "%d"}`+"`, os.Getpid()))")
		},
		// The page already wildcards the height, so there is nothing to repair.
		brokenUnedited: false,
		edits: []edit{{
			name: "no edit needed",
			apply: func(t *testing.T, page string) string {
				return page
			},
			wantExit: 0, staysGreen: true,
		}},
	},
	{
		name: "added field",
		mutate: func(t *testing.T, src string) string {
			return replaceOnce(t, src, `"symbol": "DEMO"}`, `"symbol": "DEMO", "decimals": "6"}`)
		},
		brokenUnedited: true,
		edits: []edit{{
			name: "add the new field to the expected output",
			apply: func(t *testing.T, page string) string {
				return replaceOnce(t, page, `"symbol": "DEMO"}`, `"symbol": "DEMO", "decimals": "6"}`)
			},
			wantExit: 0, staysGreen: true,
		}},
	},
}

func TestMatrix(t *testing.T) {
	page := readFile(t, pagePath)
	cli := readFile(t, cliPath)

	for _, r := range matrix {
		t.Run(r.name, func(t *testing.T) {
			mutated := r.mutate(t, cli)

			// The mutation must actually break (or, for noise, not break) the
			// page as written, or the row is testing nothing.
			ok, out := runPage(t, page, mutated)
			if ok == r.brokenUnedited {
				t.Fatalf("unedited page against the mutated CLI: passed=%v, want passed=%v\n%s", ok, !r.brokenUnedited, out)
			}

			for _, e := range r.edits {
				t.Run(e.name, func(t *testing.T) {
					edited := e.apply(t, page)

					code, verdicts := review(t, page, edited)
					if code != e.wantExit {
						t.Fatalf("docci review exited %d, want %d\n%s", code, e.wantExit, verdicts)
					}

					green, out := runPage(t, edited, mutated)
					if green != e.staysGreen {
						t.Fatalf("edited page against the mutated CLI: passed=%v, want passed=%v\n%s", green, e.staysGreen, out)
					}
				})
			}
		})
	}
}
