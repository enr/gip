package main

import (
	"fmt"
)

// Exit statuses. Scripts and CI can tell "the repositories need attention"
// (1) from "gip was called or configured wrongly" (2).
const (
	exitOK      = 0
	exitFailure = 1 // at least one project failed (or is missing, with --strict)
	exitUsage   = 2 // configuration not found or invalid, bad flags or arguments
)

// usageErrorf is the error of a command that cannot start because of how it
// was called or configured: it exits with exitUsage.
func usageErrorf(template string, args ...interface{}) error {
	return exitError(exitUsage, fmt.Sprintf(template, args...))
}

// Skip reasons that classify a skipped project (see skipKindOf). The other
// reasons come from filters and mean "nothing to do".
const (
	reasonNotProjectDir = "not a project dir"
	reasonDirMissing    = "local dir missing"
	reasonBranchMissing = "branch not found locally"
	reasonPullNever     = "pull_policy: never"
	reasonUncommitted   = "uncommitted changes (use --force)"
)

// skipKind groups the reasons a project was skipped for.
type skipKind string

const (
	// skipMissing: the configured directory or branch does not exist. It
	// usually means a wrong configuration; --strict turns it into an error.
	skipMissing skipKind = "missing"
	// skipProtected: gip refused to touch the repository on purpose
	// (pull_policy: never, uncommitted changes without --force).
	skipProtected skipKind = "protected"
	// skipFiltered: nothing to do (clean, in sync, excluded by a filter).
	skipFiltered skipKind = "filtered"
)

var skipKinds = []skipKind{skipMissing, skipProtected, skipFiltered}

func skipKindOf(reason string) skipKind {
	switch reason {
	case reasonNotProjectDir, reasonDirMissing, reasonBranchMissing:
		return skipMissing
	case reasonPullNever, reasonUncommitted:
		return skipProtected
	default:
		return skipFiltered
	}
}

// strictResult turns a missing project into an error when --strict is set:
// in CI a wrong path must fail the run instead of passing as "skipped".
func strictResult(r opResult) opResult {
	if strictMode && r.status == opSkipped && skipKindOf(r.reason) == skipMissing {
		r.status = opError
		r.err = fmt.Errorf("%s (--strict)", r.reason)
	}
	return r
}
