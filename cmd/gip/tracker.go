package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/enr/gip/lib/core"
)

var (
	styleOK      = lipgloss.NewStyle().Foreground(lipgloss.Green)
	styleErr     = lipgloss.NewStyle().Foreground(lipgloss.Red)
	styleSkip    = lipgloss.NewStyle().Foreground(lipgloss.Yellow)
	styleMag     = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	styleWhite   = lipgloss.NewStyle().Foreground(lipgloss.White)
	styleDivider = lipgloss.NewStyle().Faint(true)
)

type opStatus int

const (
	opOK opStatus = iota
	opError
	opSkipped
)

type opResult struct {
	project    string
	localPath  string // resolved path; empty if resolution failed
	status     opStatus
	err        error
	reason     string     // human-readable reason for opSkipped
	branch     string     // set by doBranch
	state      *repoState // nil when the repository state was not read
	commitMsg  string
	commitDate string
	// latest tag info (set by doReleases)
	tagSet          bool
	lastTag         string
	commitsSinceTag int
	tagDate         string
	unreleased      bool
	unreachableTags []string
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
	Name       string `json:"name"`
	LocalPath  string `json:"local_path"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Ahead      int    `json:"ahead,omitempty"`
	Behind     int    `json:"behind,omitempty"`
	NoRemote   bool   `json:"no_remote,omitempty"`
	Dirty      string `json:"dirty,omitempty"`
	CommitMsg  string `json:"commit_msg,omitempty"`
	CommitDate string `json:"commit_date,omitempty"`
	// releases command: CommitsSinceTag is a pointer so that 0 (HEAD on the
	// tag) is still emitted.
	LastTag         string `json:"last_tag,omitempty"`
	CommitsSinceTag *int   `json:"commits_since_tag,omitempty"`
	TagDate         string `json:"tag_date,omitempty"`
	Unreleased      bool   `json:"unreleased,omitempty"`
	// tags not reachable from HEAD that descend from LastTag (see releases)
	UnreachableTags []string `json:"unreachable_tags,omitempty"`
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
	Error      string   `json:"error,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Ahead      int      `json:"ahead,omitempty"`
	Behind     int      `json:"behind,omitempty"`
	NoRemote   bool     `json:"no_remote,omitempty"`
	Dirty      string   `json:"dirty,omitempty"`
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
	mu              sync.Mutex  // guards results slice
	outMu           *sync.Mutex // serialises all terminal writes; shared with core output funcs
	results         []opResult
	total           int
	started         time.Time
	tty             bool // stderr is a TTY: gates the progress bar
	colorTTY        bool // stdout is a TTY: gates ANSI styling of summary/tables
	lastProgressLen int  // display-column width of the last progress line; guarded by outMu
}

func newTracker(total int) *tracker {
	return &tracker{
		outMu:    &sync.Mutex{},
		total:    total,
		started:  time.Now(),
		tty:      isTTY(os.Stderr),
		colorTTY: isTTY(os.Stdout),
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
	total := t.total
	t.mu.Unlock()
	if !quietMode && !jsonMode {
		t.drawProgress(done, total, r.project)
	}
}

// expect adds n results to the expected total (extra per-branch results).
func (t *tracker) expect(n int) {
	t.mu.Lock()
	t.total += n
	t.mu.Unlock()
}

func (t *tracker) drawProgress(done, total int, name string) {
	if !t.tty || total == 0 {
		return
	}
	if done > total {
		total = done
	}
	filled := done * 20 / total
	pct := done * 100 / total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	line := fmt.Sprintf("[%s] %d/%d (%d%%) — %s", bar, done, total, pct, name)
	cols := len([]rune(line))
	t.outMu.Lock()
	padding := t.lastProgressLen - cols
	if padding < 0 {
		padding = 0
	}
	fmt.Fprintf(os.Stderr, "\r%s%s", line, strings.Repeat(" ", padding))
	t.lastProgressLen = cols
	t.outMu.Unlock()
}

// clearProgressLocked erases the progress bar. outMu must already be held.
func (t *tracker) clearProgressLocked() {
	if !t.tty {
		return
	}
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", t.lastProgressLen))
	t.lastProgressLen = 0
}

func (t *tracker) clearProgress() {
	t.outMu.Lock()
	t.clearProgressLocked()
	t.outMu.Unlock()
}

// printSummary prints the text summary. Always writes to stdout regardless of quiet mode.
func (t *tracker) printSummary(errorsLast, showSkipped bool) {
	t.clearProgress()

	var okCount, errCount, skipCount int
	var errEntries, skipEntries []opResult
	for _, r := range t.results {
		switch r.status {
		case opOK:
			okCount++
		case opError:
			errCount++
			errEntries = append(errEntries, r)
		case opSkipped:
			skipCount++
			skipEntries = append(skipEntries, r)
		}
	}

	elapsed := time.Since(t.started)
	okStr := t.styleIf(okCount > 0, styleOK, fmt.Sprintf("OK: %d", okCount))
	errStr := t.styleIf(errCount > 0, styleErr, fmt.Sprintf("Errors: %d", errCount))
	skipStr := t.styleIf(skipCount > 0, styleSkip, fmt.Sprintf("Skipped: %d", skipCount))
	fmt.Fprintf(os.Stdout, "%s\n", t.applyStyle(styleDivider, "─────────────────────────────────────────"))
	fmt.Fprintf(os.Stdout, "%s   %s   %s   Duration: %.1fs\n", okStr, errStr, skipStr, elapsed.Seconds())
	if noopMode {
		fmt.Fprintf(os.Stdout, "DRY-RUN — no operations performed. Remove --noop to proceed.\n")
	}
	if errorsLast && len(errEntries) > 0 {
		fmt.Fprintf(os.Stdout, "\n%s\n", t.applyStyle(styleDivider, "── Errors ────────────────────────────────"))
		for _, r := range errEntries {
			fmt.Fprintf(os.Stdout, "%s %s — %v\n", t.applyStyle(styleErr, "[ERR]"), r.project, r.err)
		}
	}
	if showSkipped && len(skipEntries) > 0 {
		fmt.Fprintf(os.Stdout, "\n%s\n", t.applyStyle(styleDivider, "── Skipped ───────────────────────────────"))
		for _, r := range skipEntries {
			fmt.Fprintf(os.Stdout, "%s %s — %s\n", t.applyStyle(styleSkip, "[SKIP]"), r.project, r.reason)
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
			Name:       r.project,
			LocalPath:  r.localPath,
			Status:     r.statusString(),
			Reason:     r.reason,
			Branch:     r.branch,
			CommitMsg:  r.commitMsg,
			CommitDate: r.commitDate,
		}
		if r.err != nil {
			p.Error = r.err.Error()
		}
		if r.state != nil {
			p.Ahead = r.state.sync.Ahead
			p.Behind = r.state.sync.Behind
			p.NoRemote = r.state.sync.NoRemote
			p.Dirty = r.state.dirty.Symbols()
		}
		if r.tagSet {
			commits := r.commitsSinceTag
			p.LastTag = r.lastTag
			p.CommitsSinceTag = &commits
			p.TagDate = r.tagDate
			p.Unreleased = r.unreleased
			p.UnreachableTags = r.unreachableTags
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

// applyStyle renders text with a lipgloss style, but only when writing to a TTY
// and not in JSON mode. Falls back to plain text otherwise.
func (t *tracker) applyStyle(s lipgloss.Style, text string) string {
	if !t.colorTTY || jsonMode {
		return text
	}
	return s.Render(text)
}

func (t *tracker) styleIf(cond bool, s lipgloss.Style, text string) string {
	if !cond {
		return text
	}
	return t.applyStyle(s, text)
}

// colorSync styles the sync label by state (TTY only): green synced, magenta
// ahead, yellow behind, red diverged, white no upstream. dirty marks a synced
// label with "*": "synced" must not suggest nothing to do on a dirty tree.
func (t *tracker) colorSync(s core.BranchSyncStatus, dirty bool) string {
	label := s.SyncLabel()
	switch {
	case s.NoRemote:
		return t.applyStyle(styleWhite, label)
	case s.Ahead == 0 && s.Behind == 0:
		if dirty {
			label += "*"
		}
		return t.applyStyle(styleOK, label)
	case s.Ahead > 0 && s.Behind > 0:
		return t.applyStyle(styleErr, label)
	case s.Ahead > 0:
		return t.applyStyle(styleMag, label)
	default:
		return t.applyStyle(styleSkip, label)
	}
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
