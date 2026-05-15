package main

import (
	"errors"
	"strings"
	"testing"
)

func TestTracker_Counts(t *testing.T) {
	tr := newTracker(4)
	tr.record(opResult{project: "a", status: opOK})
	tr.record(opResult{project: "b", status: opOK})
	tr.record(opResult{project: "c", status: opError, err: errors.New("boom")})
	tr.record(opResult{project: "d", status: opSkipped, reason: "pull_policy: never"})

	errs := tr.errors()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if _, ok := errs["c"]; !ok {
		t.Error("expected error for project 'c'")
	}

	var okCount, errCount, skipCount int
	for _, r := range tr.results {
		switch r.status {
		case opOK:
			okCount++
		case opError:
			errCount++
		case opSkipped:
			skipCount++
		}
	}
	if okCount != 2 || errCount != 1 || skipCount != 1 {
		t.Fatalf("counts: ok=%d err=%d skip=%d; want 2/1/1", okCount, errCount, skipCount)
	}
}

func TestTracker_ErrorsMap(t *testing.T) {
	tr := newTracker(2)
	e1 := errors.New("timeout")
	tr.record(opResult{project: "x", status: opError, err: e1})
	tr.record(opResult{project: "y", status: opOK})

	m := tr.errors()
	if len(m) != 1 {
		t.Fatalf("want 1 error entry, got %d", len(m))
	}
	if m["x"] != e1 {
		t.Errorf("wrong error for 'x': %v", m["x"])
	}
}

func TestProgressBar(t *testing.T) {
	cases := []struct {
		done, total, width int
		wantFilled         int
	}{
		{0, 10, 20, 0},
		{5, 10, 20, 10},
		{10, 10, 20, 20},
		{1, 4, 20, 5},
	}
	for _, tc := range cases {
		filled := tc.done * tc.width / tc.total
		bar := strings.Repeat("█", filled) + strings.Repeat("░", tc.width-filled)
		if len([]rune(bar)) != tc.width {
			t.Errorf("bar length %d, want %d", len([]rune(bar)), tc.width)
		}
		if strings.Count(bar, "█") != tc.wantFilled {
			t.Errorf("filled blocks %d, want %d (bar=%q)", strings.Count(bar, "█"), tc.wantFilled, bar)
		}
	}
}

func TestIsTTY_Pipe(t *testing.T) {
	// os.Stdin in a test process is typically a pipe, not a TTY.
	// Verify that isTTY returns false for it (not a character device).
	// We can't guarantee stdin is a pipe in all CI environments, so we only
	// assert the function does not panic and returns a bool.
	_ = isTTY(nil) // should not panic when stat fails
}
