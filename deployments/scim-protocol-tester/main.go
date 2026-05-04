package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const usage = `scim-protocol-tester — YAML-driven SCIM 2.0 protocol exerciser.

Usage:
  scim-protocol-tester [flags] <scenario-file-or-dir> [more files...]

When a directory is supplied, every *.yaml and *.yml file inside it is
loaded recursively. Scenarios run sequentially; the tool exits 1 as
soon as any scenario fails (so it composes cleanly with CI).

Flags:
`

func main() {
	timeout := flag.Duration("timeout", 30*time.Second, "per-request HTTP timeout")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	files, err := collectScenarioFiles(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no scenario files found")
		os.Exit(2)
	}

	runner := &Runner{
		Client: &http.Client{Timeout: *timeout},
		Stdout: os.Stdout,
	}

	ctx := context.Background()
	failed := 0
	for _, f := range files {
		sc, err := loadScenario(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✘ %s — load error: %v\n", f, err)
			failed++
			continue
		}
		fmt.Printf("▶ %s\n", scenarioLabel(f, sc))
		res, err := runner.Run(ctx, sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✘ runner error: %v\n", err)
			failed++
			continue
		}
		if res.Failed() {
			failed++
		}
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d scenario(s) failed\n", failed)
		os.Exit(1)
	}
	fmt.Printf("\nall %d scenario(s) passed\n", len(files))
}

// loadScenario reads and YAML-decodes one file.
func loadScenario(path string) (Scenario, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // tester reads scenarios named on the CLI
	if err != nil {
		return Scenario{}, err
	}
	var sc Scenario
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		return Scenario{}, fmt.Errorf("parse yaml: %w", err)
	}
	if sc.Target == "" {
		return Scenario{}, errors.New("scenario.target is required")
	}
	if len(sc.Steps) == 0 {
		return Scenario{}, errors.New("scenario.scenario must contain at least one step")
	}
	return sc, nil
}

// collectScenarioFiles walks each input. Files are returned as-is;
// directories are scanned recursively for *.yaml and *.yml. The result
// is sorted for deterministic order across CI runs.
func collectScenarioFiles(inputs []string) ([]string, error) {
	var out []string
	for _, in := range inputs {
		info, err := os.Stat(in)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", in, err)
		}
		if !info.IsDir() {
			out = append(out, in)
			continue
		}
		err = filepath.WalkDir(in, func(p string, d os.DirEntry, werr error) error {
			if werr != nil {
				return werr
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext == ".yaml" || ext == ".yml" {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %q: %w", in, err)
		}
	}
	sort.Strings(out)
	return out, nil
}

func scenarioLabel(path string, sc Scenario) string {
	if sc.Name != "" {
		return fmt.Sprintf("%s (%s)", sc.Name, path)
	}
	return path
}
