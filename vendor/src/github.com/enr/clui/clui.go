package clui

/*
Clui
Minimalistic UI for Golang command line apps.
*/

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-colorable"
)

type VerbosityLevel int8

const (
	VerbosityLevelMute VerbosityLevel = iota - 1
	// warn and error
	VerbosityLevelLow
	// lifecycle, success, warn and error
	VerbosityLevelMedium
	VerbosityLevelHigh
)

type MessageCategory uint

const (
	MessageCategoryConfidential MessageCategory = 10
	MessageCategoryError                        = 20
	MessageCategoryQuestion                     = 32
)

type UiColor uint

const (
	UiColorNone    UiColor = 0
	UiColorRed             = 31
	UiColorGreen           = 32
	UiColorYellow          = 33
	UiColorBlue            = 34
	UiColorMagenta         = 35
	UiColorCyan            = 36
	UiColorGray            = 37
)

type Layout interface {
	Apply(MessageCategory, string) string
	SupportsColors() bool
}

type PlainLayout struct{}

func (l *PlainLayout) Apply(category MessageCategory, message string) string {
	return fmt.Sprintf("%s\n", message)
}
func (l *PlainLayout) SupportsColors() bool {
	return true
}

type MachineReadableLayout struct{}

func (l *MachineReadableLayout) Apply(category MessageCategory, message string) string {
	now := time.Now().UTC()
	message = strings.Replace(message, ",", "+cluicomma+", -1)
	message = strings.Replace(message, "\r", "\\r", -1)
	message = strings.Replace(message, "\n", "\\n", -1)
	categoryLabel := GetCategoryLabel(category)
	return fmt.Sprintf("%d,%s,%s\n", now.Unix(), categoryLabel, message)
}
func (l *MachineReadableLayout) SupportsColors() bool {
	return false
}

type Clui struct {
	Layout         Layout
	VerbosityLevel VerbosityLevel
	Interactive    bool
	Color          bool
	Reader         io.Reader
	StdWriter      io.Writer
	ErrorWriter    io.Writer
}

func (c *Clui) Confidential(message string) {
	if c.VerbosityLevel < VerbosityLevelHigh {
		return
	}
	c.render(MessageCategoryConfidential, message, c.StdWriter, UiColorGray, false)
}
func (c *Clui) Confidentialf(format string, a ...interface{}) {
	c.Confidential(fmt.Sprintf(format, a...))
}
func (c *Clui) Lifecycle(message string) {
	if c.VerbosityLevel < VerbosityLevelMedium {
		return
	}
	c.render(MessageCategoryConfidential, message, c.StdWriter, UiColorNone, false)
}
func (c *Clui) Lifecyclef(format string, a ...interface{}) {
	c.Lifecycle(fmt.Sprintf(format, a...))
}
func (c *Clui) Title(message string) {
	if c.VerbosityLevel < VerbosityLevelMedium {
		return
	}
	c.render(MessageCategoryConfidential, message, c.StdWriter, UiColorGray, true)
}
func (c *Clui) Titlef(format string, a ...interface{}) {
	c.Title(fmt.Sprintf(format, a...))
}
func (c *Clui) Warn(message string) {
	if c.VerbosityLevel < VerbosityLevelLow {
		return
	}
	writer := c.ErrorWriter
	if writer == nil {
		writer = c.StdWriter
	}
	c.render(MessageCategoryConfidential, message, writer, UiColorYellow, false)
}
func (c *Clui) Warnf(format string, a ...interface{}) {
	c.Warn(fmt.Sprintf(format, a...))
}
func (c *Clui) Error(message string) {
	if c.VerbosityLevel < VerbosityLevelLow {
		return
	}
	writer := c.ErrorWriter
	if writer == nil {
		writer = c.StdWriter
	}
	c.render(MessageCategoryConfidential, message, writer, UiColorRed, false)
}
func (c *Clui) Errorf(format string, a ...interface{}) {
	c.Error(fmt.Sprintf(format, a...))
}
func (c *Clui) Success(message string) {
	if c.VerbosityLevel < VerbosityLevelMedium {
		return
	}
	c.render(MessageCategoryConfidential, message, c.StdWriter, UiColorGreen, false)
}
func (c *Clui) Successf(format string, a ...interface{}) {
	c.Success(fmt.Sprintf(format, a...))
}
func (c *Clui) render(category MessageCategory, message string, writer io.Writer, color UiColor, bold bool) {
	line := c.Layout.Apply(category, message)
	if color != UiColorNone && c.Layout.SupportsColors() && c.canColorize() {
		line = c.colorize(line, color, bold)
	}
	fmt.Fprint(writer, line)
}
func (c *Clui) canColorize() bool {
	// if c.color == false return false
	// Never use colors if we have this environmental variable
	if os.Getenv("UI_NO_COLOR") != "" {
		return false
	}
	// Using go-colorable we assume it just works!
	return true
}
func (c *Clui) colorize(message string, color UiColor, bold bool) string {
	attr := 0
	if bold {
		attr = 1
	}
	return fmt.Sprintf("\033[%d;%dm%s\033[0m", attr, color, message)
}

// Question to user.
// Returns an empty string if called in no-interactive mode.
func (c *Clui) Question(query string) (string, error) {
	if !c.Interactive {
		c.render(MessageCategoryQuestion, query, c.StdWriter, UiColorNone, true)
		c.render(MessageCategoryQuestion, "leaving the question unanswered", c.StdWriter, UiColorNone, true)
		return "", nil
	}
	return c.promptUser(query, c.Reader, c.StdWriter)
}

func (c *Clui) QuestionWithDefault(query string, defaultValue string) (string, error) {
	queryPlusDefault := fmt.Sprintf("%s [%s]", query, defaultValue)
	if !c.Interactive {
		c.render(MessageCategoryQuestion, queryPlusDefault, c.StdWriter, UiColorNone, true)
		c.render(MessageCategoryQuestion, "using default value "+defaultValue, c.StdWriter, UiColorNone, true)
		return defaultValue, nil
	}
	value, err := c.promptUser(queryPlusDefault, c.Reader, c.StdWriter)
	if value == "" {
		return defaultValue, err
	}
	return value, err
}

func (c *Clui) promptUser(query string, input io.Reader, output io.Writer) (string, error) {
	if _, err := fmt.Fprint(output, query+" "); err != nil {
		return "", err
	}
	var line string
	//fmt.Fscan(input, &line)
	if _, err := fmt.Fscanln(input, &line); err != nil {
		// if user skip question using ENTER:
		// scan err: unexpected newline
		//fmt.Printf("scan err: %s\n", err)
		return line, err
	}
	return line, nil
}

func DefaultClui() *Clui {
	return &Clui{
		Layout:         &PlainLayout{},
		VerbosityLevel: VerbosityLevelHigh,
		Interactive:    true,
		Color:          true,
		Reader:         os.Stdin,
		StdWriter:      colorable.NewColorableStdout(),
		ErrorWriter:    colorable.NewColorableStderr(),
	}
}

func NewClui(options ...func(*Clui)) (*Clui, error) {
	ui := DefaultClui()
	for _, option := range options {
		option(ui)
	}
	return ui, nil
}

func GetCategoryLabel(category MessageCategory) string {
	switch category {
	case MessageCategoryConfidential:
		return "info"
	case MessageCategoryQuestion:
		return "question"
	case MessageCategoryError:
		return "error"
	default:
		return ""
	}
}
