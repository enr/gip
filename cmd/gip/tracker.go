package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
)

type opStatus int

const (
	opOK opStatus = iota
	opError
	opSkipped
)

type opResult struct {
	project   string
	localPath string // resolved path; empty if resolution failed
	status    opStatus
	err       error
	reason    string // human-readable reason for opSkipped
	branch    string // set by doBranch
}

func (r opResult) statusString() string {
	switch r.status {
	case opOK:
		return "ok"
	case opError:
		return "error"
	case opSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// --- JSON output types ---

type projectJSON struct {
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Branch    string `json:"branch,omitempty"`
}

type summaryJSON struct {
	Total      int   `json:"total"`
	OK         int   `json:"ok"`
	Errors     int   `json:"errors"`
	Skipped    int   `json:"skipped"`
	DurationMs int64 `json:"duration_ms"`
}

type commandOutputJSON struct {
	Command   string        `json:"command"`
	Timestamp string        `json:"timestamp"`
	Projects  []projectJSON `json:"projects"`
	Summary   summaryJSON   `json:"summary"`
	Warnings  []string      `json:"warnings"`
}

type listProjectJSON struct {
	Name       string   `json:"name"`
	LocalPath  string   `json:"local_path"`
	Repository string   `json:"repository"`
	Policy     string   `json:"policy"`
	Provider   string   `json:"provider"`
	Tags       []string `json:"tags"`
	Missing    bool     `json:"missing,omitempty"`
	Disabled   bool     `json:"disabled,omitempty"`
}

type listOutputJSON struct {
	Command   string            `json:"command"`
	Timestamp string            `json:"timestamp"`
	Projects  []listProjectJSON `json:"projects"`
	Warnings  []string          `json:"warnings"`
}

// --- tracker ---

// tracker records per-project outcomes, drives the progress bar on stderr
// (TTY only), and renders either a text summary or JSON output at the end.
type tracker struct {
	mu      sync.Mutex  // guards results slice
	outMu   *sync.Mutex // serialises all terminal writes; shared with core output funcs
	results []opResult
	total   int
	started time.Time
	tty     bool
}

func newTracker(total int) *tracker {
	return &tracker{
		outMu:   &sync.Mutex{},
		total:   total,
		started: time.Now(),
		tty:     isTTY(os.Stderr),
	}
}

// printNoop writes a dry-run line to stdout in a thread-safe way.
func (t *tracker) printNoop(format string, args ...interface{}) {
	t.outMu.Lock()
	fmt.Fprintf(os.Stdout, "[DRY-RUN] "+format+"\n", args...)
	t.outMu.Unlock()
}

// withOutput clears the progress bar, runs fn (which writes project output to
// stdout), then releases the output lock. Concurrent calls are serialised so
// that progress-bar redraws never interleave with project output.
func (t *tracker) withOutput(fn func()) {
	t.outMu.Lock()
	t.clearProgressLocked()
	fn()
	t.outMu.Unlock()
}

// sharedOutput returns the output mutex and a clear-bar callback for use by
// core.WithSharedOutput so that git/exec display sections share the same lock.
func (t *tracker) sharedOutput() (*sync.Mutex, func()) {
	return t.outMu, t.clearProgressLocked
}

// record stores one project result and redraws the progress bar.
func (t *tracker) record(r opResult) {
	t.mu.Lock()
	t.results = append(t.results, r)
	done := len(t.results)
	t.mu.Unlock()
	if !quietMode && !jsonMode {
		t.drawProgress(done, r.project)
	}
}

func (t *tracker) drawProgress(done int, name string) {
	if !t.tty || t.total == 0 {
		return
	}
	filled := done * 20 / t.total
	pct := done * 100 / t.total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	t.outMu.Lock()
	fmt.Fprintf(os.Stderr, "\r[%s] %d/%d (%d%%) — %s   ", bar, done, t.total, pct, name)
	t.outMu.Unlock()
}

// clearProgressLocked erases the progress bar. outMu must already be held.
func (t *tracker) clearProgressLocked() {
	if !t.tty {
		return
	}
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 80))
}

func (t *tracker) clearProgress() {
	t.outMu.Lock()
	t.clearProgressLocked()
	t.outMu.Unlock()
}

// printSummary prints the text summary. Always writes to stdout regardless of quiet mode.
func (t *tracker) printSummary(errorsLast bool) {
	t.clearProgress()

	var okCount, errCount, skipCount int
	var errEntries []opResult
	for _, r := range t.results {
		switch r.status {
		case opOK:
			okCount++
		case opError:
			errCount++
			errEntries = append(errEntries, r)
		case opSkipped:
			skipCount++
		}
	}

	elapsed := time.Since(t.started)
	okStr := t.color(ansiGreen, fmt.Sprintf("OK: %d", okCount))
	errStr := t.color(ansiRed, fmt.Sprintf("Errors: %d", errCount))
	skipStr := t.color(ansiYellow, fmt.Sprintf("Skipped: %d", skipCount))
	fmt.Fprintf(os.Stdout, "─────────────────────────────────────────\n")
	fmt.Fprintf(os.Stdout, "%s   %s   %s   Duration: %.1fs\n", okStr, errStr, skipStr, elapsed.Seconds())
	if noopMode {
		fmt.Fprintf(os.Stdout, "DRY-RUN — no operations performed. Remove --noop to proceed.\n")
	}
	if errorsLast && len(errEntries) > 0 {
		fmt.Fprintf(os.Stdout, "\n── Errors ────────────────────────────────\n")
		for _, r := range errEntries {
			fmt.Fprintf(os.Stdout, "%s %s — %v\n", t.color(ansiRed, "[ERR]"), r.project, r.err)
		}
	}
}

// printJSON serialises all results as a JSON envelope and writes to stdout.
func (t *tracker) printJSON(command string, warnings []string) {
	t.clearProgress()

	projects := make([]projectJSON, 0, len(t.results))
	var okCount, errCount, skipCount int
	for _, r := range t.results {
		p := projectJSON{
			Name:      r.project,
			LocalPath: r.localPath,
			Status:    r.statusString(),
			Reason:    r.reason,
			Branch:    r.branch,
		}
		if r.err != nil {
			p.Error = r.err.Error()
		}
		projects = append(projects, p)
		switch r.status {
		case opOK:
			okCount++
		case opError:
			errCount++
		case opSkipped:
			skipCount++
		}
	}
	if warnings == nil {
		warnings = []string{}
	}
	out := commandOutputJSON{
		Command:   command,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Projects:  projects,
		Summary: summaryJSON{
			Total:      t.total,
			OK:         okCount,
			Errors:     errCount,
			Skipped:    skipCount,
			DurationMs: time.Since(t.started).Milliseconds(),
		},
		Warnings: warnings,
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

// errors returns collected errors as a map suitable for buildError.
func (t *tracker) errors() map[string]error {
	m := make(map[string]error)
	for _, r := range t.results {
		if r.status == opError {
			m[r.project] = r.err
		}
	}
	return m
}

func (t *tracker) color(code, text string) string {
	if !t.tty || jsonMode {
		return text
	}
	return code + text + ansiReset
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}
