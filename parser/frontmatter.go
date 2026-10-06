package parser

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// PageConfig is the page-level docci configuration, read from the `docci:` key
// of a page's YAML frontmatter. Mintlify ignores frontmatter keys it does not
// recognise, so this costs nothing in the rendered page.
type PageConfig struct {
	// Runnable opts a page out of execution entirely. A page carrying docci
	// directives is runnable by default; set `runnable: false` for a page whose
	// blocks are illustrative.
	Runnable *bool `yaml:"runnable"`

	// Needs lists pages this one builds on, as paths relative to the repository
	// root or to this page. They are run, in order, before this page, in the
	// same shell.
	Needs []string `yaml:"needs"`

	// Cleanup runs after the page finishes, pass or fail. It is not instruction
	// to the reader, so it does not belong in a step.
	Cleanup []string `yaml:"cleanup"`

	// WorkingDir changes directory before the page's first block.
	WorkingDir string `yaml:"working-dir"`
}

// IsRunnable reports whether the page opted out of execution.
func (p PageConfig) IsRunnable() bool {
	return p.Runnable == nil || *p.Runnable
}

type frontmatterDoc struct {
	Docci *PageConfig `yaml:"docci"`
}

// SplitFrontmatter separates a YAML frontmatter block from the body of a
// markdown or MDX document. It returns the page config, the body, and the
// number of lines the frontmatter occupied, so that line numbers reported
// against the body still point at the right line of the original file.
//
// A document with no frontmatter is returned unchanged with a zero config.
func SplitFrontmatter(document string) (PageConfig, string, int, error) {
	lines := splitIntoLines(document)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return PageConfig{}, document, 0, nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		// An unterminated `---` is not frontmatter. Treat the whole document as body.
		return PageConfig{}, document, 0, nil
	}

	raw := strings.Join(lines[1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")

	var doc frontmatterDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return PageConfig{}, body, end + 1, fmt.Errorf("parse frontmatter: %w", err)
	}
	if doc.Docci == nil {
		return PageConfig{}, body, end + 1, nil
	}
	return *doc.Docci, body, end + 1, nil
}
