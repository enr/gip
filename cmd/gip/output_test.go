package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func testCommands() []*cli.Command {
	return []*cli.Command{
		{Name: "status", Aliases: []string{"s"}, Flags: []cli.Flag{
			&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}},
			&cli.StringSliceFlag{Name: "tag"},
			&cli.BoolFlag{Name: "dirty"},
		}},
		{Name: "exec", Flags: []cli.Flag{
			&cli.IntFlag{Name: "jobs", Aliases: []string{"j"}},
		}},
		{Name: "init", Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force"},
			// Same name as a global flag: the command's one wins.
			&cli.BoolFlag{Name: "quiet"},
		}},
	}
}

func testGlobals() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "file", Aliases: []string{"f"}},
		&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}},
		&cli.BoolFlag{Name: "json"},
	}
}

func TestHoistGlobalFlags(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nothing to do", []string{"gip", "status"}, []string{"gip", "status"}},
		{"already before", []string{"gip", "--json", "status"}, []string{"gip", "--json", "status"}},
		{"bool after command", []string{"gip", "status", "--json"}, []string{"gip", "--json", "status"}},
		{"alias after command", []string{"gip", "s", "-q", "--dirty"}, []string{"gip", "-q", "s", "--dirty"}},
		{"value flag after command", []string{"gip", "status", "-f", "x.yml", "-j", "2"}, []string{"gip", "-f", "x.yml", "status", "-j", "2"}},
		{"value with equals", []string{"gip", "status", "--file=x.yml"}, []string{"gip", "--file=x.yml", "status"}},
		{"command flag values skipped", []string{"gip", "status", "-j", "8", "--tag", "work", "--json"}, []string{"gip", "--json", "status", "-j", "8", "--tag", "work"}},
		{"global value before command", []string{"gip", "-f", "status", "exec", "--json"}, []string{"gip", "-f", "status", "--json", "exec"}},
		{"stops at --", []string{"gip", "exec", "-j", "2", "--", "git", "log", "-q", "--json"}, []string{"gip", "exec", "-j", "2", "--", "git", "log", "-q", "--json"}},
		{"hoists before --", []string{"gip", "exec", "--json", "--", "make", "-q"}, []string{"gip", "--json", "exec", "--", "make", "-q"}},
		{"stops at positional", []string{"gip", "exec", "git", "status", "-q"}, []string{"gip", "exec", "git", "status", "-q"}},
		{"command flag wins", []string{"gip", "init", "--quiet", "--json"}, []string{"gip", "--json", "init", "--quiet"}},
		{"unknown command", []string{"gip", "stauts", "--json"}, []string{"gip", "stauts", "--json"}},
		{"unknown flag left", []string{"gip", "status", "--nope", "--json"}, []string{"gip", "--json", "status", "--nope"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hoistGlobalFlags(tc.in, testGlobals(), testCommands())
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("hoistGlobalFlags(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestErrorSummaryLine(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("exit status 1"), "exit status 1"},
		{
			errors.New("exit status 128: Cloning into '/x'...\nfatal: could not read Username for 'https://github.com': terminal prompts disabled\n"),
			"exit status 128: fatal: could not read Username for 'https://github.com': terminal prompts disabled (hint: authentication required: configure a git credential helper or use an SSH URL)",
		},
		{errors.New("exit status 2: first\nsecond\n\n"), "exit status 2: second"},
		{errors.New("ssh: connect\nfatal: Could not resolve host: nowhere"), "ssh: fatal: Could not resolve host: nowhere (hint: host not resolved: check the repository URL and the network)"},
	}
	for _, tc := range cases {
		if got := errorSummaryLine(tc.err); got != tc.want {
			t.Errorf("errorSummaryLine(%q)\n got: %q\nwant: %q", tc.err, got, tc.want)
		}
		if strings.Contains(errorSummaryLine(tc.err), "\n") {
			t.Errorf("errorSummaryLine(%q) is not a single line", tc.err)
		}
	}
	if errorSummaryLine(nil) != "" {
		t.Error("nil error must give an empty line")
	}
}

func TestColorEnabled(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	if colorEnabled(f) {
		t.Error("a regular file is not a terminal: no colors expected")
	}
	t.Setenv("FORCE_COLOR", "1")
	if !colorEnabled(f) {
		t.Error("FORCE_COLOR must enable colors")
	}
	t.Setenv("NO_COLOR", "1")
	if colorEnabled(f) {
		t.Error("NO_COLOR must win over FORCE_COLOR")
	}
}

func TestANSIStripWriter(t *testing.T) {
	var buf bytes.Buffer
	w := ansiStripWriter{w: &buf}
	in := "\x1b[0;33mwarning\n\x1b[0m"
	n, err := w.Write([]byte(in))
	if err != nil || n != len(in) {
		t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(in))
	}
	if buf.String() != "warning\n" {
		t.Fatalf("got %q", buf.String())
	}
}

// TestCLIOutputHygiene runs the binary with output captured (not a terminal):
// no ANSI codes, no provider noise, no configuration banner for -f, global
// flags accepted after the command, usage errors with status 2.
func TestCLIOutputHygiene(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)

	missing := filepath.Join(tmpDir, "missing")
	configPath := filepath.Join(tmpDir, "gip.yml")
	content := "- name: selfhosted\n  repository: \"https://git.internal.example/team/app.git\"\n  local_path: " + missing + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("GIP_FILE", "")

	run := func(args ...string) (string, string, int) { return runGip(t, binPath, args...) }

	stdout, stderr, code := run("-f", configPath, "status")
	if code != 0 {
		t.Fatalf("status exit %d\n%s%s", code, stdout, stderr)
	}
	if strings.Contains(stdout+stderr, "\x1b[") {
		t.Errorf("ANSI codes written to a non-terminal:\n%q\n%q", stdout, stderr)
	}
	if strings.Contains(stderr, "provider") {
		t.Errorf("provider notice must be shown only with --debug:\n%s", stderr)
	}
	if strings.Contains(stderr, "using configuration") {
		t.Errorf("no configuration banner expected with -f:\n%s", stderr)
	}
	if !strings.Contains(stderr, "not a project dir") {
		t.Errorf("the missing directory warning must still be printed:\n%s", stderr)
	}

	// Global flag after the command.
	stdout, stderr, code = run("status", "-f", configPath, "--json")
	if code != 0 || !strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Fatalf("status -f ... --json: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// Unknown command: one line, suggestion, status 2.
	_, stderr, code = run("-f", configPath, "stauts")
	if code != 2 || !strings.Contains(stderr, `unknown command "stauts"`) || !strings.Contains(stderr, `"status"`) {
		t.Fatalf("unknown command: exit %d\n%s", code, stderr)
	}

}

// TestCLIConfigurationErrors checks that configuration errors are reported once
// and point to the way out.
func TestCLIConfigurationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildBinary(t, tmpDir)
	t.Setenv("GIP_FILE", "")

	// Missing configuration: the way out is in the message.
	t.Setenv("HOME", tmpDir)
	cmd := exec.Command(binPath, "status")
	cmd.Dir = tmpDir
	out, _ := cmd.CombinedOutput()
	if !bytes.Contains(out, []byte("gip init")) {
		t.Fatalf("missing configuration must point to 'gip init':\n%s", out)
	}

	// Invalid configuration: reported once.
	bad := filepath.Join(tmpDir, "bad.yml")
	if err := os.WriteFile(bad, []byte("- name: x\n  local_path: [\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runGip(t, binPath, "-f", bad, "list")
	if code != 1 || strings.Count(stderr, "did not find expected node content") != 1 {
		t.Fatalf("invalid configuration: exit %d\n%s", code, stderr)
	}
}

// runGip runs the gip binary and returns its stdout, stderr and exit status.
func runGip(t *testing.T, binPath string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}
