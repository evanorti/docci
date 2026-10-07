package review

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The safety property: for any page and any edit to it, PairAssertions either
// refuses, or pairs each assertion only with the same logical assertion in the
// other version, and drops none. Silent mispairing is the only forbidden
// outcome.
//
// The generator tracks every block by a stable id that survives edits, so the
// test knows which assertion is which. The seeded loop is deterministic, and
// the shrinker reduces any failure to a small case before reporting it.

// propBlock is one block of a generated page.
type propBlock struct {
	id       int
	name     string // "" means the author gave no name
	heading  string // "" means the block sits under no heading
	contains bool
	expect   bool
	plain    bool // a bash block with no assertion
	version  int  // bumped by an in-place edit
}

// propOp is one edit applied to the base page to produce the head page.
type propOp struct {
	op    string // insert, remove, edit, rename, reorder
	id    int    // the block the op targets; for insert, the new block's id
	block propBlock
	name  string
	index int
}

type propCase struct {
	base []propBlock
	ops  []propOp
}

var (
	propNames    = []string{"", "", "", "", "alpha", "beta", "assertion #1", "assertion #2"}
	propHeadings = []string{"", "", "", "Setup", "Mint", "assertion #1"}
	propOps      = []string{"insert", "remove", "edit", "rename", "reorder"}
)

func genBlock(r *rand.Rand, id int) propBlock {
	b := propBlock{
		id:      id,
		name:    propNames[r.Intn(len(propNames))],
		heading: propHeadings[r.Intn(len(propHeadings))],
	}
	if r.Intn(6) == 0 {
		b.plain = true
		return b
	}
	switch r.Intn(3) {
	case 0:
		b.contains = true
	case 1:
		b.expect = true
	default:
		b.contains, b.expect = true, true
	}
	return b
}

func genCase(r *rand.Rand) propCase {
	var c propCase
	nextID := 1
	for n := 1 + r.Intn(6); n > 0; n-- {
		c.base = append(c.base, genBlock(r, nextID))
		nextID++
	}
	for n := r.Intn(6); n > 0; n-- {
		op := propOp{op: propOps[r.Intn(len(propOps))], id: 1 + r.Intn(nextID), index: r.Intn(7)}
		switch op.op {
		case "insert":
			op.id = nextID
			op.block = genBlock(r, nextID)
			nextID++
		case "rename":
			op.name = propNames[r.Intn(len(propNames))]
		}
		c.ops = append(c.ops, op)
	}
	return c
}

// applyOps returns the head version. An op naming a block that is no longer
// present does nothing, so shrinking can drop blocks without invalidating ops.
func applyOps(base []propBlock, ops []propOp) []propBlock {
	blocks := append([]propBlock(nil), base...)
	for _, op := range ops {
		i := indexOf(blocks, op.id)
		switch op.op {
		case "insert":
			at := min(op.index, len(blocks))
			blocks = append(blocks[:at], append([]propBlock{op.block}, blocks[at:]...)...)
		case "remove":
			if i >= 0 {
				blocks = append(blocks[:i], blocks[i+1:]...)
			}
		case "edit":
			if i >= 0 {
				blocks[i].version++
			}
		case "rename":
			if i >= 0 {
				blocks[i].name = op.name
			}
		case "reorder":
			if i >= 0 {
				moved := blocks[i]
				blocks = append(blocks[:i], blocks[i+1:]...)
				at := min(op.index, len(blocks))
				blocks = append(blocks[:at], append([]propBlock{moved}, blocks[at:]...)...)
			}
		}
	}
	return blocks
}

func indexOf(blocks []propBlock, id int) int {
	for i, b := range blocks {
		if b.id == id {
			return i
		}
	}
	return -1
}

// renderPage writes blocks as a page. Each assertion block gets its name and
// output-contains directives, and its expect-output directive when it has one.
// Text embeds the block id and version, so the test can recover the block from
// an extracted assertion.
func renderPage(blocks []propBlock) string {
	var lines []string
	for _, b := range blocks {
		if b.heading != "" {
			lines = append(lines, "## "+b.heading, "")
		}
		if b.plain {
			lines = append(lines, "```bash", "echo plain", "```", "")
			continue
		}
		var directives []string
		if b.name != "" {
			directives = append(directives, fmt.Sprintf("name=%q", b.name))
		}
		if b.contains {
			directives = append(directives, fmt.Sprintf("output-contains=\"out-%d-%d\"", b.id, b.version))
		}
		if len(directives) > 0 {
			lines = append(lines, "<!-- docci "+strings.Join(directives, " ")+" -->", "")
		}
		lines = append(lines, "```bash", fmt.Sprintf("echo block-%d", b.id), "```", "")
		if b.expect {
			lines = append(lines,
				"<!-- docci expect-output -->", "",
				"```json", fmt.Sprintf("{\"id\": %d, \"v\": %d}", b.id, b.version), "```", "")
		}
	}
	return strings.Join(lines, "\n")
}

var idInText = regexp.MustCompile(`out-(\d+)-\d+|"id": (\d+)`)

func idOf(text string) (int, bool) {
	m := idInText.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	raw := m[1]
	if raw == "" {
		raw = m[2]
	}
	id, err := strconv.Atoi(raw)
	return id, err == nil
}

// declared is the identity the design promises: the author's name, then the
// heading the block falls under, and only then the block itself. The heading is
// the nearest one above the block, because the parser gives every block the
// heading in effect at that point, not only the block that carries the heading.
func declared(b propBlock, heading string) string {
	switch {
	case b.name != "":
		return "name:" + b.name
	case heading != "":
		return "heading:" + heading
	default:
		return fmt.Sprintf("block:%d", b.id)
	}
}

// logicalAssertion is one assertion as the generator knows it.
type logicalAssertion struct {
	decl string
	kind string
	id   int
}

func logicalOf(blocks []propBlock) []logicalAssertion {
	var out []logicalAssertion
	heading := ""
	for _, b := range blocks {
		if b.heading != "" {
			heading = b.heading
		}
		if b.plain {
			continue
		}
		if b.contains {
			out = append(out, logicalAssertion{declared(b, heading), "contains", b.id})
		}
		if b.expect {
			out = append(out, logicalAssertion{declared(b, heading), "expect", b.id})
		}
	}
	return out
}

func findLogical(list []logicalAssertion, id int, kind string) (logicalAssertion, bool) {
	for _, l := range list {
		if l.id == id && l.kind == kind {
			return l, true
		}
	}
	return logicalAssertion{}, false
}

// hasDuplicateIdentity reports whether two logical assertions share a declared
// identity and kind. PairAssertions must refuse such a page.
func hasDuplicateIdentity(list []logicalAssertion) bool {
	seen := map[string]bool{}
	for _, l := range list {
		k := l.decl + "\x00" + l.kind
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

// checkCase returns a description of the violation, if any. skipped is true when
// the generated page did not parse, which is a generator problem, not a guard
// result. A refusal from PairAssertions is always acceptable.
func checkCase(c propCase) (violation string, skipped bool) {
	head := applyOps(c.base, c.ops)
	before, err := Extract(renderPage(c.base), "page.md")
	if err != nil {
		return "", true
	}
	after, err := Extract(renderPage(head), "page.md")
	if err != nil {
		return "", true
	}

	pairs, err := PairAssertions(before, after)
	if err != nil {
		return "", false
	}

	baseLog := logicalOf(c.base)
	headLog := logicalOf(head)
	if hasDuplicateIdentity(baseLog) || hasDuplicateIdentity(headLog) {
		return "a result was returned although two assertions share an identity", false
	}

	beforeCovered := map[string]bool{}
	afterCovered := map[string]bool{}
	for _, p := range pairs {
		var before, after *logicalAssertion
		if p.Before != nil {
			id, ok := idOf(p.Before.Text)
			if !ok {
				return fmt.Sprintf("cannot read the block id from base text %q", p.Before.Text), false
			}
			l, ok := findLogical(baseLog, id, p.Kind)
			if !ok {
				return fmt.Sprintf("pair %s/%s claims a base assertion the generator never wrote", p.Key, p.Kind), false
			}
			before = &l
			beforeCovered[l.decl+"\x00"+l.kind] = true
		}
		if p.After != nil {
			id, ok := idOf(p.After.Text)
			if !ok {
				return fmt.Sprintf("cannot read the block id from head text %q", p.After.Text), false
			}
			l, ok := findLogical(headLog, id, p.Kind)
			if !ok {
				return fmt.Sprintf("pair %s/%s claims a head assertion the generator never wrote", p.Key, p.Kind), false
			}
			after = &l
			afterCovered[l.decl+"\x00"+l.kind] = true
		}
		if before != nil && after != nil && before.decl != after.decl {
			return fmt.Sprintf("MISPAIR: %s/%s joins base block %s to head block %s", p.Key, p.Kind, before.decl, after.decl), false
		}
	}

	for _, l := range baseLog {
		if !beforeCovered[l.decl+"\x00"+l.kind] {
			return fmt.Sprintf("DROPPED: base assertion %s/%s is in no pair", l.decl, l.kind), false
		}
	}
	for _, l := range headLog {
		if !afterCovered[l.decl+"\x00"+l.kind] {
			return fmt.Sprintf("DROPPED: head assertion %s/%s is in no pair", l.decl, l.kind), false
		}
	}
	return "", false
}

func describeCase(c propCase) string {
	var b strings.Builder
	b.WriteString("base blocks:\n")
	for _, blk := range c.base {
		fmt.Fprintf(&b, "  %s\n", describeBlock(blk))
	}
	b.WriteString("ops:\n")
	for _, op := range c.ops {
		switch op.op {
		case "insert":
			fmt.Fprintf(&b, "  insert at %d: %s\n", op.index, describeBlock(op.block))
		case "rename":
			fmt.Fprintf(&b, "  rename block %d to %q\n", op.id, op.name)
		case "reorder":
			fmt.Fprintf(&b, "  reorder block %d to index %d\n", op.id, op.index)
		default:
			fmt.Fprintf(&b, "  %s block %d\n", op.op, op.id)
		}
	}
	fmt.Fprintf(&b, "base page:\n%s\n", renderPage(c.base))
	fmt.Fprintf(&b, "head page:\n%s\n", renderPage(applyOps(c.base, c.ops)))
	return b.String()
}

func describeBlock(b propBlock) string {
	return fmt.Sprintf("block %d name=%q heading=%q contains=%v expect=%v plain=%v version=%d",
		b.id, b.name, b.heading, b.contains, b.expect, b.plain, b.version)
}

// shrink drops ops and base blocks one at a time, keeping any removal that still
// fails, until no single removal does.
func shrink(c propCase) propCase {
	fails := func(candidate propCase) bool {
		v, skipped := checkCase(candidate)
		return v != "" && !skipped
	}
	for changed := true; changed; {
		changed = false
		for i := range c.ops {
			candidate := propCase{base: c.base, ops: append(append([]propOp(nil), c.ops[:i]...), c.ops[i+1:]...)}
			if fails(candidate) {
				c, changed = candidate, true
				break
			}
		}
		if changed {
			continue
		}
		for i := range c.base {
			candidate := propCase{base: append(append([]propBlock(nil), c.base[:i]...), c.base[i+1:]...), ops: c.ops}
			if fails(candidate) {
				c, changed = candidate, true
				break
			}
		}
	}
	return c
}

func TestPairPropertyPairsCorrectlyOrRefuses(t *testing.T) {
	const cases = 3000
	skipped := 0
	for seed := int64(1); seed <= cases; seed++ {
		c := genCase(rand.New(rand.NewSource(seed)))
		violation, skip := checkCase(c)
		if skip {
			skipped++
			continue
		}
		if violation == "" {
			continue
		}
		small := shrink(c)
		smallViolation, _ := checkCase(small)
		t.Fatalf("seed %d: %s\nshrunk to: %s\n%s", seed, violation, smallViolation, describeCase(small))
	}
	if skipped > cases/10 {
		t.Fatalf("%d of %d generated pages did not parse; the generator is producing pages the parser rejects", skipped, cases)
	}
	t.Logf("checked %d generated cases, %d skipped as unparseable", cases, skipped)
}
