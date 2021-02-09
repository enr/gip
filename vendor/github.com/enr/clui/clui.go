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

// VerbosityLevel of messages
type VerbosityLevel int8

const (
	// VerbosityLevelMute mutes all messages
	VerbosityLevelMute VerbosityLevel = iota - 1
	// VerbosityLevelLow allows warn and error
	VerbosityLevelLow
	// VerbosityLevelMedium allows lifecycle, success, warn and error
	VerbosityLevelMedium
	// VerbosityLevelHigh allows all messages
	VerbosityLevelHigh
)

// MessageCategory classify the message
type MessageCategory int8

const (
	// MessageCategoryConfidential confidential messages
	MessageCategoryConfidential MessageCategory = iota - 1
	// MessageCategoryInfo info
	MessageCategoryInfo
	// MessageCategoryWarn warning
	MessageCategoryWarn
	// MessageCategoryError error
	MessageCategoryError
	// MessageCategoryQuestion question
	MessageCategoryQuestion
)

// UIColor represents color
type UIColor uint

const (
	// UIColorNone no color
	UIColorNone UIColor = 0
	// UIColorRed red
	UIColorRed = 31
	// UIColorGreen green
	UIColorGreen = 32
	// UIColorYellow yellow
	UIColorYellow = 33
	// UIColorBlue blue
	UIColorBlue = 34
	// UIColorMagenta magenta
	UIColorMagenta = 35
	// UIColorCyan cyan
	UIColorCyan = 36
	// UIColorGray gray
	UIColorGray = 37
)

// Layout of the final message
type Layout interface {
	// Apply merge category and text.
	Apply(MessageCategory, string) string
	// SupportsColors returns if the Layout can render colored messages in the console.
	SupportsColors() bool
}

// PlainLayout is the defaut and simplest Layout
type PlainLayout struct{}

// Apply returns the text message string
func (l *PlainLayout) Apply(category MessageCategory, message string) string {
	return fmt.Sprintf("%s\n", message)
}

// SupportsColors returns always true
func (l *PlainLayout) SupportsColors() bool {
	return true
}

// MachineReadableLayout layout for programmatic access
type MachineReadableLayout struct{}

// Apply returns a comma separated string
func (l *MachineReadableLayout) Apply(category MessageCategory, message string) string {
	now := time.Now().UTC()
	message = strings.Replace(message, ",", "+cluicomma+", -1)
	message = strings.Replace(message, "\r", "\\r", -1)
	message = strings.Replace(message, "\n", "\\n", -1)
	categoryLabel := l.getCategoryLabel(category)
	return fmt.Sprintf("%d,%s,%s\n", now.Unix(), categoryLabel, message)
}

// SupportsColors return always false
func (l *MachineReadableLayout) SupportsColors() bool {
	return false
}

func (l *MachineReadableLayout) getCategoryLabel(category MessageCategory) string {
	switch category {
	case MessageCategoryConfidential:
		return "debug"
	case MessageCategoryInfo:
		return "info"
	case MessageCategoryWarn:
		return "warn"
	case MessageCategoryQuestion:
		return "question"
	case MessageCategoryError:
		return "error"
	default:
		return ""
	}
}

// Clui is the actual message writer.
type Clui struct {
	Layout         Layout
	VerbosityLevel VerbosityLevel
	Interactive    bool
	Color          bool
	Reader         io.Reader
	StdWriter      io.Writer
	ErrorWriter    io.Writer
}

// Confidential write a confidential message
func (c *Clui) Confidential(message string) {
	if c.VerbosityLevel < VerbosityLevelHigh {
		return
	}
	c.render(MessageCategoryConfidential, message, c.StdWriter, UIColorGray, false)
}

// Confidentialf write a confidential message
func (c *Clui) Confidentialf(format string, a ...interface{}) {
	c.Confidential(fmt.Sprintf(format, a...))
}

// Lifecycle write a info message
func (c *Clui) Lifecycle(message string) {
	if c.VerbosityLevel < VerbosityLevelMedium {
		return
	}
	c.render(MessageCategoryInfo, message, c.StdWriter, UIColorNone, false)
}

// Lifecyclef write a info message
func (c *Clui) Lifecyclef(format string, a ...interface{}) {
	c.Lifecycle(fmt.Sprintf(format, a...))
}

// Title write a title message
func (c *Clui) Title(message string) {
	if c.VerbosityLevel < VerbosityLevelMedium {
		return
	}
	c.render(MessageCategoryInfo, message, c.StdWriter, UIColorGray, true)
}

// Titlef write a title message
func (c *Clui) Titlef(format string, a ...interface{}) {
	c.Title(fmt.Sprintf(format, a...))
}

// Warn write a warn message
func (c *Clui) Warn(message string) {
	if c.VerbosityLevel < VerbosityLevelLow {
		return
	}
	writer := c.ErrorWriter
	if writer == nil {
		writer = c.StdWriter
	}
	c.render(MessageCategoryWarn, message, writer, UIColorYellow, false)
}

// Warnf write a warn message
func (c *Clui) Warnf(format string, a ...interface{}) {
	c.Warn(fmt.Sprintf(format, a...))
}

// Error write a error message
func (c *Clui) Error(message string) {
	if c.VerbosityLevel < VerbosityLevelLow {
		return
	}
	writer := c.ErrorWriter
	if writer == nil {
		writer = c.StdWriter
	}
	c.render(MessageCategoryError, message, writer, UIColorRed, false)
}

// Errorf write a error message
func (c *Clui) Errorf(format string, a ...interface{}) {
	c.Error(fmt.Sprintf(format, a...))
}

// Success write a success message
func (c *Clui) Success(message string) {
	if c.VerbosityLevel < VerbosityLevelMedium {
		return
	}
	c.render(MessageCategoryInfo, message, c.StdWriter, UIColorGreen, false)
}

// Successf write a success message
func (c *Clui) Successf(format string, a ...interface{}) {
	c.Success(fmt.Sprintf(format, a...))
}

func (c *Clui) render(category MessageCategory, message string, writer io.Writer, color UIColor, bold bool) {
	line := c.Layout.Apply(category, message)
	if color != UIColorNone && c.Layout.SupportsColors() && c.canColorize() {
		line = c.colorize(line, color, bold)
	}
	fmt.Fprint(writer, line)
}

func (c *Clui) canColorize() bool {
	if c.Color == false {
		return false
	}
	// Never use colors if we have this environmental variable
	if os.Getenv("UI_NO_COLOR") != "" {
		return false
	}
	// Using go-colorable we assume it just works!
	return true
}

func (c *Clui) colorize(message string, color UIColor, bold bool) string {
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
		c.render(MessageCategoryQuestion, query, c.StdWriter, UIColorNone, true)
		c.render(MessageCategoryQuestion, "leaving the question unanswered", c.StdWriter, UIColorNone, true)
		return "", nil
	}
	return c.promptUser(query, c.Reader, c.StdWriter)
}

// QuestionWithDefault is a question to user with default value.
// Returns the default value if called in no-interactive mode.
func (c *Clui) QuestionWithDefault(query string, defaultValue string) (string, error) {
	queryPlusDefault := fmt.Sprintf("%s [%s]", query, defaultValue)
	if !c.Interactive {
		c.render(MessageCategoryQuestion, queryPlusDefault, c.StdWriter, UIColorNone, true)
		c.render(MessageCategoryQuestion, "using default value "+defaultValue, c.StdWriter, UIColorNone, true)
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

// DefaultClui is the factory function for a Clui with default configuration.
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

// NewClui is the factory function allowing to configure the Clui.
func NewClui(options ...func(*Clui)) (*Clui, error) {
	ui := DefaultClui()
	for _, option := range options {
		option(ui)
	}
	return ui, nil
}
