// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package validate

import (
	"os"
	"testing"
	"time"

	"github.com/aireilly/nasc-cli/internal/model"
)

func doc() model.Doc {
	return model.Doc{
		Fields: map[string]model.Value{
			"description": {Kind: model.KindString, Str: "Load when touching auth and sessions here."},
			"type":        {Kind: model.KindString, Str: "architecture"},
		},
	}
}

func TestExists(t *testing.T) {
	ok, err := EvalIn("exists(description)", EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("exists(description) = %v, %v", ok, err)
	}
	ok, _ = EvalIn("exists(missing)", EvalContext{Doc: doc(), Today: time.Now()})
	if ok {
		t.Fatalf("exists(missing) should be false")
	}
}

func TestLengthComparison(t *testing.T) {
	ok, err := EvalIn("length(description) >= 30", EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("length >= 30 = %v, %v", ok, err)
	}
	ok, _ = EvalIn("length(description) >= 500", EvalContext{Doc: doc(), Today: time.Now()})
	if ok {
		t.Fatalf("length >= 500 should be false")
	}
}

func TestFieldEquals(t *testing.T) {
	ok, err := EvalIn(`type == "architecture"`, EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf(`type == "architecture" = %v, %v`, ok, err)
	}
}

func TestMalformedStringLiteral(t *testing.T) {
	// Single quote character should error, not panic
	ok, err := EvalIn(`type == "`, EvalContext{Doc: doc(), Today: time.Now()})
	if err == nil {
		t.Fatalf("malformed string literal should error, got ok=%v", ok)
	}
}

func TestContainsTrue(t *testing.T) {
	ok, err := EvalIn(`contains(description, "auth")`, EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("contains true = %v, %v", ok, err)
	}
}

func TestContainsFalse(t *testing.T) {
	ok, err := EvalIn(`contains(description, "xyz")`, EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || ok {
		t.Fatalf("contains false = %v, %v", ok, err)
	}
}

func TestContainsWithEmbeddedOperator(t *testing.T) {
	// Test that operators in quoted strings don't split the expression
	d := model.Doc{
		Fields: map[string]model.Value{
			"note": {Kind: model.KindString, Str: "score > 5 is good"},
		},
	}
	ok, err := EvalIn(`contains(note, "score > 5")`, EvalContext{Doc: d, Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("contains with embedded > = %v, %v", ok, err)
	}
}

func TestFieldNotEquals(t *testing.T) {
	ok, err := EvalIn(`type != "other"`, EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("!= true = %v, %v", ok, err)
	}
}

func TestNumericLessThan(t *testing.T) {
	ok, err := EvalIn("length(description) < 100", EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("< true = %v, %v", ok, err)
	}
	ok, _ = EvalIn("length(description) < 10", EvalContext{Doc: doc(), Today: time.Now()})
	if ok {
		t.Fatalf("< false should be false")
	}
}

func TestNumericLessThanOrEqual(t *testing.T) {
	ok, err := EvalIn("length(description) <= 42", EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("<= true = %v, %v", ok, err)
	}
}

func TestNumericGreaterThan(t *testing.T) {
	ok, err := EvalIn("length(description) > 30", EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("> true = %v, %v", ok, err)
	}
}

func TestMissingFieldComparison(t *testing.T) {
	// Missing field in ordering comparison should return false, not error
	ok, err := EvalIn("missing_field >= 30", EvalContext{Doc: doc(), Today: time.Now()})
	if err != nil {
		t.Fatalf("missing field comparison should not error: %v", err)
	}
	if ok {
		t.Fatalf("missing field comparison should be false")
	}
}

func TestDateComparison(t *testing.T) {
	// Fixed date for deterministic testing
	fixedDate := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)

	d := model.Doc{
		Fields: map[string]model.Value{
			"lastUpdated": {Kind: model.KindDate, Str: "2026-08-30"},
		},
	}

	// Fresh date (one day old) should fail >= today()
	ok, err := EvalIn("lastUpdated >= today()", EvalContext{Doc: d, Root: ".", Today: fixedDate})
	if err != nil || ok {
		t.Fatalf("stale date >= today() should be false, got %v, %v", ok, err)
	}

	// Today's date should pass >= today()
	d.Fields["lastUpdated"] = model.Value{Kind: model.KindDate, Str: "2026-08-31"}
	ok, err = EvalIn("lastUpdated >= today()", EvalContext{Doc: d, Root: ".", Today: fixedDate})
	if err != nil || !ok {
		t.Fatalf("today's date >= today() should be true, got %v, %v", ok, err)
	}

	// Future date should pass >= today()
	d.Fields["lastUpdated"] = model.Value{Kind: model.KindDate, Str: "2026-09-01"}
	ok, err = EvalIn("lastUpdated >= today()", EvalContext{Doc: d, Root: ".", Today: fixedDate})
	if err != nil || !ok {
		t.Fatalf("future date >= today() should be true, got %v, %v", ok, err)
	}
}

func TestPathMatchCount(t *testing.T) {
	// Create a temporary directory structure
	tmpdir := t.TempDir()

	// Create some test files
	if err := os.WriteFile(tmpdir+"/file1.txt", []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpdir+"/file2.txt", []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(tmpdir+"/subdir", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpdir+"/subdir/file3.txt", []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	d := model.Doc{
		Fields: map[string]model.Value{},
		Paths:  []string{"*.txt"},
	}

	ok, err := EvalIn("path_match_count(paths) >= 2", EvalContext{Doc: d, Root: tmpdir, Today: time.Now()})
	if err != nil || !ok {
		t.Fatalf("path_match_count >= 2 = %v, %v", ok, err)
	}
}
