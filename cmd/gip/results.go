package main

import (
	"context"
	"strconv"
	"strings"

	"github.com/enr/gip/lib/core"
)

// statusFile is one changed file reported by "git status --porcelain".
type statusFile struct {
	Path     string
	OrigPath string // source of a rename or copy
	Index    string // X letter, "." when unchanged
	Worktree string // Y letter, "." when unchanged
}

// parsePorcelain parses "git status --porcelain" (v1) output. It always
// returns a non-nil slice: an empty one means a clean tree.
func parsePorcelain(out string) []statusFile {
	files := []statusFile{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		f := statusFile{Index: statusLetter(line[0]), Worktree: statusLetter(line[1])}
		path := line[3:]
		if (line[0] == 'R' || line[0] == 'C') && strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			f.OrigPath = unquotePath(parts[0])
			path = parts[1]
		}
		f.Path = unquotePath(path)
		files = append(files, f)
	}
	return files
}

func statusLetter(b byte) string {
	if b == ' ' {
		return "."
	}
	return string(b)
}

// unquotePath decodes a path that git quoted because of special characters
// ("caf\303\251.txt").
func unquotePath(p string) string {
	if len(p) >= 2 && strings.HasPrefix(p, `"`) && strings.HasSuffix(p, `"`) {
		if u, err := strconv.Unquote(p); err == nil {
			return u
		}
	}
	return p
}

// pullInfo is what a pull changed: HEAD before and after, and the number of
// commits between them. from is empty for a clone or an unborn branch.
type pullInfo struct {
	cloned  bool
	from    string
	to      string
	commits int
}

// headOrEmpty returns HEAD, or "" when it cannot be read (unborn branch).
func headOrEmpty(ctx context.Context, git *core.GitCommands, dir string) string {
	h, err := git.Head(ctx, dir)
	if err != nil {
		return ""
	}
	return h
}

// readPullInfo completes the information of a successful pull or clone that
// started from HEAD from ("" when unknown).
func readPullInfo(ctx context.Context, git *core.GitCommands, dir, from string, cloned bool) *pullInfo {
	info := &pullInfo{cloned: cloned, from: from, to: headOrEmpty(ctx, git, dir)}
	if info.from != "" && info.to != "" && info.from != info.to {
		if n, err := git.CountCommits(ctx, dir, info.from, info.to); err == nil {
			info.commits = n
		}
	}
	return info
}
