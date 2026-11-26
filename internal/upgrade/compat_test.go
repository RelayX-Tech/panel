package upgrade_test

import (
	"testing"

	"github.com/relayx-tech/panel/internal/upgrade"
)

func TestParseAndCompare(t *testing.T) {
	a, err := upgrade.ParseVersion("1.4.0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	b, _ := upgrade.ParseVersion("1.3.9")
	if a.Compare(b) <= 0 {
		t.Fatalf("1.4.0 should be greater than 1.3.9")
	}
	if _, err := upgrade.ParseVersion("v1.4"); err == nil {
		t.Fatal("invalid version should fail")
	}
}

func TestCheckUpgradeTarget(t *testing.T) {
	if err := upgrade.CheckTarget("1.3.0", "1.4.0"); err != nil {
		t.Fatalf("upgrade should be allowed: %v", err)
	}
	if err := upgrade.CheckTarget("1.4.0", "1.3.0"); err != upgrade.ErrDowngrade {
		t.Fatalf("downgrade err = %v", err)
	}
	if err := upgrade.CheckTarget("1.4.0", "2.0.0"); err != upgrade.ErrMajorMismatch {
		t.Fatalf("major mismatch err = %v", err)
	}
	if err := upgrade.CheckTarget("1.4.0", "1.4.0"); err != upgrade.ErrSameVersion {
		t.Fatalf("same version err = %v", err)
	}
}

func TestCheckSchemaAndContract(t *testing.T) {
	if err := upgrade.CheckCompatibility(1, 1, "1", "1", true); err != nil {
		t.Fatalf("compatible: %v", err)
	}
	if err := upgrade.CheckCompatibility(1, 2, "1", "1", true); err != upgrade.ErrContractMajor {
		t.Fatalf("contract major err = %v", err)
	}
	if err := upgrade.CheckCompatibility(1, 1, "1", "1", false); err != upgrade.ErrSchemaMismatch {
		t.Fatalf("schema err = %v", err)
	}
	if err := upgrade.CheckCompatibility(1, 1, "1", "2", true); err != upgrade.ErrAgentMinorAhead {
		t.Fatalf("agent minor err = %v", err)
	}
}
