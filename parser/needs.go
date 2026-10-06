package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// pageExtensions are the extensions tried when a `needs` entry omits one, so a
// page can reference another by the path the docs site uses.
var pageExtensions = []string{"", ".mdx", ".md"}

// ResolveRunOrder expands the pages given on the command line into the full
// ordered list of pages to run, with each page's prerequisites in front of it.
//
// A page that builds on another declares it rather than duplicating its setup.
// Running the real prerequisite is slower than a hand-written setup script, but
// a setup script is a second source of truth for how you reach that state, and
// nothing tells you when it has drifted from the page that teaches it.
func ResolveRunOrder(paths []string) ([]string, map[string]PageConfig, error) {
	var order []string
	seen := make(map[string]bool)
	configs := make(map[string]PageConfig)

	var visit func(path string, chain []string) error
	visit = func(path string, chain []string) error {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		for _, ancestor := range chain {
			if ancestor == abs {
				return fmt.Errorf("pages depend on each other in a cycle: %s", describeCycle(append(chain, abs)))
			}
		}

		if seen[abs] {
			return nil
		}

		document, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		cfg, _, _, err := SplitFrontmatter(string(document))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		for _, need := range cfg.Needs {
			resolved, err := resolveNeed(need, abs)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if err := visit(resolved, append(chain, abs)); err != nil {
				return err
			}
		}

		seen[abs] = true
		configs[abs] = cfg
		order = append(order, path)
		return nil
	}

	for _, path := range paths {
		if err := visit(path, nil); err != nil {
			return nil, nil, err
		}
	}

	return order, configs, nil
}

// resolveNeed turns a `needs` entry into a path on disk. An entry starting with
// `/` is resolved from the repository root, the way a docs site addresses its
// own pages; anything else is relative to the page that declared it.
func resolveNeed(need string, fromPage string) (string, error) {
	var base string
	if strings.HasPrefix(need, "/") {
		root, err := repositoryRoot(filepath.Dir(fromPage))
		if err != nil {
			return "", fmt.Errorf("needs %q is repository-root relative, but no repository root was found: %w", need, err)
		}
		base = filepath.Join(root, need)
	} else {
		base = filepath.Join(filepath.Dir(fromPage), need)
	}

	for _, ext := range pageExtensions {
		candidate := base + ext
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("needs %q, which does not exist (tried %s)", need, strings.Join(pageExtensions[1:], ", "))
}

// repositoryRoot walks up from dir looking for a .git directory.
func repositoryRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("reached the filesystem root without finding .git")
		}
		dir = parent
	}
}

func describeCycle(chain []string) string {
	names := make([]string, 0, len(chain))
	for _, path := range chain {
		names = append(names, filepath.Base(path))
	}
	return strings.Join(names, " -> ")
}
