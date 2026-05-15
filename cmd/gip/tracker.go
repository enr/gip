package main

import (
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
	project string
	status  opStatus
	err     error
	reason  string // human-readable reason for opSkipped
}

// tracker records per-project outcomes, draws a progress bar on stderr (TTY
// only), and prints a summary after all operations complete.
type tracker struct {
	mu      sync.Mutex
	results []opResult
	total   int
	started time.Time
	tty     bool // stderr is a character device
}

func newTracker(total int) *tracker {
	return &tracker{
		total:   total,
		started: time.Now(),
		tty:     isTTY(os.Stderr),
	}
}

// record stores the result of one project operation and redraws the progress bar.
func (t *tracker) record(r opResult) {
	t.mu.Lock()
	t.results = append(t.results, r)
	done := len(t.results)
	t.mu.Unlock()
	if !quietMode {
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
	fmt.Fprintf(os.Stderr, "\r[%s] %d/%d (%d%%) — %s   ", bar, done, t.total, pct, name)
}

func (t *tracker) clearProgress() {
	if !t.tty {
		return
	}
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 80))
}

// printSummary prints the run summary. When errorsLast is true it also prints
// a grouped error section. Always outputs to stdout regardless of quiet mode.
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
	errStr := t.color(ansiRed, fmt.Sprintf("Errori: %d", errCount))
	skipStr := t.color(ansiYellow, fmt.Sprintf("Saltati: %d", skipCount))
	fmt.Fprintf(os.Stdout, "─────────────────────────────────────────\n")
	fmt.Fprintf(os.Stdout, "%s   %s   %s   Durata: %.1fs\n", okStr, errStr, skipStr, elapsed.Seconds())

	if errorsLast && len(errEntries) > 0 {
		fmt.Fprintf(os.Stdout, "\n── Errori ────────────────────────────────\n")
		for _, r := range errEntries {
			fmt.Fprintf(os.Stdout, "%s %s — %v\n", t.color(ansiRed, "[ERR]"), r.project, r.err)
		}
	}
}

// errors returns the collected errors as a map suitable for buildError.
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
	if !t.tty {
		return text
	}
	return code + text + ansiReset
}

func isTTY(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}
