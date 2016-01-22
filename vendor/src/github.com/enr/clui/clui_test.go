package clui

import (
	"bytes"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// This reads the output from the bytes.Buffer in our test object
// and then resets the buffer.
func readWriter(c *Clui) (result string) {
	buffer := c.StdWriter.(*bytes.Buffer)
	result = buffer.String()
	buffer.Reset()
	return
}
func readErrorWriter(c *Clui) (result string) {
	buffer := c.ErrorWriter.(*bytes.Buffer)
	result = buffer.String()
	buffer.Reset()
	return
}

func testClui(level VerbosityLevel) *Clui {
	return &Clui{
		Layout:         &PlainLayout{},
		VerbosityLevel: level,
		Interactive:    true,
		Color:          true,
		Reader:         new(bytes.Buffer),
		StdWriter:      new(bytes.Buffer),
		ErrorWriter:    new(bytes.Buffer),
	}
}

func assertWriterEmpty(ui *Clui, t *testing.T) {
	result := readWriter(ui)
	if result != "" {
		t.Fatalf("std writer: expected empty but got %s", result)
	}
}
func assertErrorWriterEmpty(ui *Clui, t *testing.T) {
	result := readErrorWriter(ui)
	if result != "" {
		t.Fatalf("err writer: expected empty but got %s", result)
	}
}
func assertWriterEquals(message string, ui *Clui, t *testing.T) {
	result := readWriter(ui)
	if result != message {
		t.Fatalf("std writer: expected %s but got %s", message, result)
	}
}
func assertErrorWriterEquals(message string, ui *Clui, t *testing.T) {
	result := readErrorWriter(ui)
	if result != message {
		t.Fatalf("err writer: expected %s but got %s", message, result)
	}
}
func assertWriterContains(message string, ui *Clui, t *testing.T) {
	result := readWriter(ui)
	if !strings.Contains(result, message) {
		t.Fatalf("std writer: expected containing %s but got %s", message, result)
	}
}
func assertErrorWriterContains(message string, ui *Clui, t *testing.T) {
	result := readErrorWriter(ui)
	if !strings.Contains(result, message) {
		t.Fatalf("err writer: expected containing %s but got %s", message, result)
	}
}

func TestCluiVerbosityLevelMute(t *testing.T) {
	ui := testClui(VerbosityLevelMute)
	// success
	ui.Success("success")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// successf
	ui.Successf("successf")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// confidential
	ui.Confidential("confidential")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// confidentialf
	ui.Confidentialf("confidentialf")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// lifecycle
	ui.Lifecycle("lifecycle")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// lifecyclef
	ui.Lifecyclef("lifecyclef")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// warn
	ui.Warn("warn")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// warnf
	ui.Warnf("warnf")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// error
	ui.Error("error")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// errorf
	ui.Errorf("errorf")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
}
func TestCluiVerbosityLevelLow(t *testing.T) {
	ui := testClui(VerbosityLevelLow)
	// success
	ui.Success("success")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// successf
	ui.Successf("successf")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// confidential
	ui.Confidential("confidential")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// confidentialf
	ui.Confidentialf("confidentialf")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// lifecycle
	ui.Lifecycle("lifecycle")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// lifecyclef
	ui.Lifecycle("lifecyclef")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// warn
	ui.Warn("TestCluiVerbosityLevelLow/warn")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelLow/warn", ui, t)
	// warnf
	ui.Warnf("%s/warnf", "TestCluiVerbosityLevelLow")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelLow/warnf", ui, t)
	// error
	ui.Error("TestCluiVerbosityLevelLow/error")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelLow/error", ui, t)
	// error
	ui.Errorf("%s/errorf", "TestCluiVerbosityLevelLow")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelLow/errorf", ui, t)
}
func TestCluiVerbosityLevelMedium(t *testing.T) {
	ui := testClui(VerbosityLevelMedium)
	// success
	ui.Success("TestCluiVerbosityLevelMedium/success")
	assertWriterContains("TestCluiVerbosityLevelMedium/success", ui, t)
	assertErrorWriterEmpty(ui, t)
	// confidential
	ui.Confidential("TestCluiVerbosityLevelMedium/confidential")
	assertWriterEmpty(ui, t)
	assertErrorWriterEmpty(ui, t)
	// lifecycle
	ui.Lifecycle("TestCluiVerbosityLevelMedium/lifecycle")
	assertWriterContains("TestCluiVerbosityLevelMedium/lifecycle", ui, t)
	assertErrorWriterEmpty(ui, t)
	// warn
	ui.Warn("TestCluiVerbosityLevelMedium/warn")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelMedium/warn", ui, t)
	// error
	ui.Error("TestCluiVerbosityLevelMedium/error")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelMedium/error", ui, t)
}
func TestCluiVerbosityLevelHigh(t *testing.T) {
	ui := testClui(VerbosityLevelHigh)
	// success
	ui.Success("TestCluiVerbosityLevelHigh/success")
	assertWriterContains("TestCluiVerbosityLevelHigh/success", ui, t)
	assertErrorWriterEmpty(ui, t)
	// confidential
	ui.Confidential("TestCluiVerbosityLevelHigh/confidential")
	assertWriterContains("TestCluiVerbosityLevelHigh/confidential", ui, t)
	assertErrorWriterEmpty(ui, t)
	// lifecycle
	ui.Lifecycle("TestCluiVerbosityLevelHigh/lifecycle")
	assertWriterContains("TestCluiVerbosityLevelHigh/lifecycle", ui, t)
	assertErrorWriterEmpty(ui, t)
	// warn
	ui.Warn("TestCluiVerbosityLevelHigh/warn")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelHigh/warn", ui, t)
	// error
	ui.Error("TestCluiVerbosityLevelHigh/error")
	assertWriterEmpty(ui, t)
	assertErrorWriterContains("TestCluiVerbosityLevelHigh/error", ui, t)
}

func TestPromptUser(t *testing.T) {
	ui := testClui(VerbosityLevelLow)
	var input, output bytes.Buffer
	input.WriteString("ok")

	l, _ := ui.promptUser("?", &input, &output)
	expected := "ok"
	if l != expected {
		t.Fatalf("ask: expected '%s', got '%s'", expected, l)
	}
}

func TestQuestionNoInteractive(t *testing.T) {
	ui := &Clui{
		Layout:         &PlainLayout{},
		VerbosityLevel: VerbosityLevelHigh,
		Interactive:    false,
		Color:          true,
		Reader:         new(bytes.Buffer),
		StdWriter:      new(bytes.Buffer),
		ErrorWriter:    new(bytes.Buffer),
	}
	value, err := ui.Question("wtf?")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if value != "" {
		t.Fatalf("invalid output:'%s'", value)
	}
}

func TestQuestionWithDefaultNoInteractive(t *testing.T) {
	ui := &Clui{
		Layout:         &PlainLayout{},
		VerbosityLevel: VerbosityLevelHigh,
		Interactive:    false,
		Color:          true,
		Reader:         new(bytes.Buffer),
		StdWriter:      new(bytes.Buffer),
		ErrorWriter:    new(bytes.Buffer),
	}
	value, err := ui.QuestionWithDefault("1 or 2?", "1")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if value != "1" {
		t.Fatalf("invalid output:'%s'", value)
	}
}

type SimpleLayout struct{}

func (l *SimpleLayout) Apply(category MessageCategory, message string) string {
	return message
}
func (l *SimpleLayout) SupportsColors() bool {
	return true
}
func TestColor(t *testing.T) {
	ui := &Clui{
		Layout:         &SimpleLayout{},
		VerbosityLevel: VerbosityLevelHigh,
		Interactive:    true,
		Color:          true,
		Reader:         new(bytes.Buffer),
		StdWriter:      new(bytes.Buffer),
		ErrorWriter:    new(bytes.Buffer),
	}
	// force color
	oldenv := os.Getenv("ANSICON")
	os.Setenv("ANSICON", "1")
	defer os.Setenv("ANSICON", oldenv)
	ui.Error("foo")
	actual := readErrorWriter(ui)
	expected := "\033[0;31mfoo\033[0m"
	if actual != expected {
		b64actual := base64.StdEncoding.EncodeToString([]byte(actual))
		b64expected := base64.StdEncoding.EncodeToString([]byte(expected))
		t.Fatalf(`testcolor invalid output %s. B64 expected="%s" actual="%s"`, actual, b64expected, b64actual)
	}
}

func TestNoColor(t *testing.T) {
	ui := &Clui{
		Layout:         &SimpleLayout{},
		VerbosityLevel: VerbosityLevelHigh,
		Interactive:    true,
		Color:          true,
		Reader:         new(bytes.Buffer),
		StdWriter:      new(bytes.Buffer),
		ErrorWriter:    new(bytes.Buffer),
	}
	// force no color
	oldenv := os.Getenv("UI_NO_COLOR")
	os.Setenv("UI_NO_COLOR", "1")
	defer os.Setenv("UI_NO_COLOR", oldenv)
	ui.Error("the error message")
	result := readErrorWriter(ui)
	if strings.TrimSpace(result) != "the error message" {
		t.Fatalf("testcolor invalid output:'%s'", result)
	}
}

type CustomLayout struct{}

func (l *CustomLayout) Apply(category MessageCategory, message string) string {
	return "_" + message + "_"
}
func (l *CustomLayout) SupportsColors() bool {
	return true
}
func TestCustomLayout(t *testing.T) {
	ui := &Clui{
		Layout:         &CustomLayout{},
		VerbosityLevel: VerbosityLevelHigh,
		Interactive:    true,
		Color:          true,
		Reader:         new(bytes.Buffer),
		StdWriter:      new(bytes.Buffer),
		ErrorWriter:    new(bytes.Buffer),
	}
	ui.Lifecycle("good message")
	result := readWriter(ui)
	if result != "_good message_" {
		t.Fatalf("invalid output: %s", result)
	}
}

func TestMachineReadableLayout(t *testing.T) {
	layout := &MachineReadableLayout{}
	result := layout.Apply(MessageCategoryError, "I'm doing this...\nand this, and that")
	suffix := ",error,I'm doing this...\\nand this+cluicomma+ and that\n"
	if !strings.HasSuffix(result, suffix) {
		t.Fatalf("error/quiet invalid output: %s", result)
	}
}

func TestGetMessageCategoryLabel(t *testing.T) {
	categoryLabel := GetCategoryLabel(MessageCategoryError)
	if categoryLabel != "error" {
		t.Fatalf("invalid output: %s", categoryLabel)
	}
}
