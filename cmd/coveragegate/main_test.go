package main

import (
	"os"
	"strings"
	"testing"
)

const sampleProfile = `mode: set
github.com/relayx-tech/panel/internal/auth/auth.go:1.1,2.2 10 5
github.com/relayx-tech/panel/internal/auth/auth.go:3.1,4.2 10 5
github.com/relayx-tech/panel/internal/logic/nodes/nodes.go:1.1,2.2 10 0
github.com/relayx-tech/panel/internal/handler/auth/auth.go:1.1,2.2 8 0
github.com/relayx-tech/panel/internal/testsupport/pg.go:1.1,2.2 4 0
`

func mustParse(t *testing.T, profile string) map[string]PackageCoverage {
	t.Helper()
	pkgs, err := ParseProfile(strings.NewReader(profile), "github.com/relayx-tech/panel")
	if err != nil {
		t.Fatalf("ParseProfile() error = %v", err)
	}
	return pkgs
}

func TestParseProfileAggregatesByPackage(t *testing.T) {
	pkgs := mustParse(t, sampleProfile)

	auth, ok := pkgs["internal/auth"]
	if !ok {
		t.Fatalf("missing package internal/auth, got %v", keys(pkgs))
	}
	if auth.Covered != 20 || auth.Total != 20 {
		t.Errorf("internal/auth = %d/%d, want 20/20", auth.Covered, auth.Total)
	}
	if got := auth.Percent(); got != 100 {
		t.Errorf("internal/auth percent = %v, want 100", got)
	}

	nodes := pkgs["internal/logic/nodes"]
	if nodes.Covered != 0 || nodes.Total != 10 {
		t.Errorf("internal/logic/nodes = %d/%d, want 0/10", nodes.Covered, nodes.Total)
	}
}

func TestParseProfileRejectsUnknownLine(t *testing.T) {
	_, err := ParseProfile(strings.NewReader("mode: set\nnot a cover line\n"), "github.com/relayx-tech/panel")
	if err == nil {
		t.Fatal("ParseProfile() error = nil, want error for malformed line")
	}
}

func TestParseProfileEmptyReturnsNoPackages(t *testing.T) {
	pkgs := mustParse(t, "mode: set\n")
	if len(pkgs) != 0 {
		t.Fatalf("len(pkgs) = %d, want 0", len(pkgs))
	}
}

func TestEvaluateFailsBelowThreshold(t *testing.T) {
	pkgs := mustParse(t, sampleProfile)
	th := Thresholds{
		Default:  80,
		Exclude:  []string{"internal/handler", "internal/testsupport"},
		Packages: map[string]float64{},
	}

	failures := Evaluate(pkgs, th)

	if len(failures) != 1 {
		t.Fatalf("failures = %v, want exactly internal/logic/nodes", failures)
	}
	if failures[0].Package != "internal/logic/nodes" {
		t.Errorf("failure package = %q, want internal/logic/nodes", failures[0].Package)
	}
	if failures[0].Threshold != 80 {
		t.Errorf("failure threshold = %v, want 80", failures[0].Threshold)
	}
}

func TestEvaluateSkipsExcludedPackages(t *testing.T) {
	pkgs := mustParse(t, sampleProfile)
	th := Thresholds{
		Default: 80,
		Exclude: []string{"internal/handler/", "internal/testsupport"},
	}

	failures := Evaluate(pkgs, th)

	for _, f := range failures {
		if strings.HasPrefix(f.Package, "internal/handler") || f.Package == "internal/testsupport" {
			t.Errorf("excluded package %q must not be evaluated", f.Package)
		}
	}
	if len(failures) != 1 || failures[0].Package != "internal/logic/nodes" {
		t.Fatalf("failures = %v, want only internal/logic/nodes", failures)
	}
}

func TestEvaluateUsesPerPackageOverride(t *testing.T) {
	pkgs := mustParse(t, sampleProfile)
	th := Thresholds{
		Default:  80,
		Exclude:  []string{"internal/handler", "internal/testsupport"},
		Packages: map[string]float64{"internal/logic/nodes": 0},
	}

	failures := Evaluate(pkgs, th)
	if len(failures) != 0 {
		t.Fatalf("failures = %v, want none (nodes 0%% meets its 0%% override, not the 80%% default)", failures)
	}
}

func TestEvaluateOverrideBelowCurrentPasses(t *testing.T) {
	pkgs := map[string]PackageCoverage{
		"internal/x": {Package: "internal/x", Covered: 50, Total: 100},
	}
	th := Thresholds{Default: 80, Packages: map[string]float64{"internal/x": 40}}
	if failures := Evaluate(pkgs, th); len(failures) != 0 {
		t.Fatalf("failures = %v, want none", failures)
	}
}

func TestEvaluateNoStatementsPackagePasses(t *testing.T) {
	pkgs := map[string]PackageCoverage{
		"internal/empty": {Package: "internal/empty", Covered: 0, Total: 0},
	}
	th := Thresholds{Default: 80}
	if failures := Evaluate(pkgs, th); len(failures) != 0 {
		t.Fatalf("failures = %v, want none for package without statements", failures)
	}
}

func TestGenerateThresholdsFloorsToTenth(t *testing.T) {
	pkgs := map[string]PackageCoverage{
		"internal/a": {Package: "internal/a", Covered: 108, Total: 139}, // 77.697...
		"internal/b": {Package: "internal/b", Covered: 1, Total: 1},     // 100
	}
	th := Thresholds{Default: 80, Exclude: []string{"internal/handler"}}

	got := GenerateThresholds(pkgs, th)

	if v := got.Packages["internal/a"]; v != 77.6 {
		t.Errorf("internal/a threshold = %v, want 77.6", v)
	}
	if v := got.Packages["internal/b"]; v != 100 {
		t.Errorf("internal/b threshold = %v, want 100", v)
	}
	if len(got.Exclude) != 1 || got.Exclude[0] != "internal/handler" {
		t.Errorf("exclude not preserved: %v", got.Exclude)
	}
}

func TestGenerateThresholdsSkipsExcludedAndEmpty(t *testing.T) {
	pkgs := map[string]PackageCoverage{
		"internal/empty":                {Package: "internal/empty", Total: 0},
		"internal/handler/auth/handler": {Package: "internal/handler/auth/handler", Covered: 0, Total: 10},
		"internal/keep":                 {Package: "internal/keep", Covered: 5, Total: 10},
	}
	th := Thresholds{Default: 80, Exclude: []string{"internal/handler"}}

	got := GenerateThresholds(pkgs, th)

	if _, ok := got.Packages["internal/empty"]; ok {
		t.Error("package without statements must not be pinned")
	}
	if _, ok := got.Packages["internal/handler/auth/handler"]; ok {
		t.Error("excluded package must not be pinned")
	}
	if v := got.Packages["internal/keep"]; v != 50 {
		t.Errorf("internal/keep threshold = %v, want 50", v)
	}
}

func TestLoadAndSaveThresholdsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/coverage-thresholds.yml"
	in := Thresholds{
		Default:  80,
		Exclude:  []string{"internal/handler", "internal/testsupport"},
		Packages: map[string]float64{"internal/logic/nodes": 17.3},
	}
	if err := SaveThresholds(path, in); err != nil {
		t.Fatalf("SaveThresholds() error = %v", err)
	}
	out, err := LoadThresholds(path)
	if err != nil {
		t.Fatalf("LoadThresholds() error = %v", err)
	}
	if out.Default != in.Default || len(out.Exclude) != len(in.Exclude) || out.Packages["internal/logic/nodes"] != 17.3 {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestRunFailsOnRegression(t *testing.T) {
	dir := t.TempDir()
	profile := dir + "/coverage.out"
	if err := writeFile(profile, sampleProfile); err != nil {
		t.Fatal(err)
	}
	thresholds := dir + "/thresholds.yml"
	if err := SaveThresholds(thresholds, Thresholds{
		Default:  80,
		Exclude:  []string{"internal/handler", "internal/testsupport"},
		Packages: map[string]float64{},
	}); err != nil {
		t.Fatal(err)
	}

	code := run([]string{"-profile", profile, "-thresholds", thresholds, "-module", "github.com/relayx-tech/panel"})
	if code != 1 {
		t.Fatalf("run() = %d, want 1 (internal/logic/nodes below 80%%)", code)
	}
}

func TestRunPassesWhenAllThresholdsMet(t *testing.T) {
	dir := t.TempDir()
	profile := dir + "/coverage.out"
	if err := writeFile(profile, sampleProfile); err != nil {
		t.Fatal(err)
	}
	thresholds := dir + "/thresholds.yml"
	if err := SaveThresholds(thresholds, Thresholds{
		Default:  80,
		Exclude:  []string{"internal/handler", "internal/testsupport"},
		Packages: map[string]float64{"internal/logic/nodes": 0},
	}); err != nil {
		t.Fatal(err)
	}

	code := run([]string{"-profile", profile, "-thresholds", thresholds, "-module", "github.com/relayx-tech/panel"})
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
}

func TestRunUpdateWritesThresholds(t *testing.T) {
	dir := t.TempDir()
	profile := dir + "/coverage.out"
	if err := writeFile(profile, sampleProfile); err != nil {
		t.Fatal(err)
	}
	thresholds := dir + "/thresholds.yml"
	if err := SaveThresholds(thresholds, Thresholds{Default: 80}); err != nil {
		t.Fatal(err)
	}

	code := run([]string{"-profile", profile, "-thresholds", thresholds, "-module", "github.com/relayx-tech/panel", "-update"})
	if code != 0 {
		t.Fatalf("run(-update) = %d, want 0", code)
	}
	got, err := LoadThresholds(thresholds)
	if err != nil {
		t.Fatal(err)
	}
	if v := got.Packages["internal/logic/nodes"]; v != 0 {
		t.Errorf("internal/logic/nodes threshold = %v, want 0", v)
	}
	if v := got.Packages["internal/auth"]; v != 100 {
		t.Errorf("internal/auth threshold = %v, want 100", v)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func keys(m map[string]PackageCoverage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
