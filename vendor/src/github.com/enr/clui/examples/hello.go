package main

import (
	"fmt"

	"github.com/enr/clui"
)

type CustomLayout struct{}

func (l *CustomLayout) Apply(category clui.MessageCategory, message string) string {
	return fmt.Sprintf("Custom: %v - %s\n", category, message)
}
func (l *CustomLayout) SupportsColors() bool {
	return true
}

func main() {

	fmt.Println("====== plain, color, interactive and verbose")
	verbosity := func(ui *clui.Clui) {
		ui.VerbosityLevel = clui.VerbosityLevelHigh
	}
	ui, _ := clui.NewClui(verbosity)
	ui.Confidential("A confidential message")
	ui.Lifecycle("I'm doing this...")
	ui.Warn("This is a warn about...")
	ui.Error("Ops, an error")
	ui.Success("Yahooo! Everything OK!")
	ui.Title("This is Title!")
	value, _ := ui.QuestionWithDefault("Question with default. 1 or 2?", "1")
	ui.Warnf("Your answer: %s", value)

	fmt.Println("====== a new ui, machine readable and non interactive")
	interac := false
	conf := func(ui *clui.Clui) {
		ui.Layout = &clui.MachineReadableLayout{}
		ui.Interactive = interac
	}
	ui, _ = clui.NewClui(conf)
	ui.Lifecycle("I'm doing this...\nand this, and that")
	value, _ = ui.QuestionWithDefault("Question with default. 3 or 4?", "4")
	ui.Warnf("Your answer: %s", value)
	ui.Title("This is Title!")

	fmt.Println("====== custom layout")
	custom := func(ui *clui.Clui) {
		ui.Layout = &CustomLayout{}
	}
	ui, _ = clui.NewClui(custom)
	ui.Success("Success!")
	ui.Title("This is Title!")

	fmt.Println("====== default Clui")
	ui = clui.DefaultClui()
	ui.Success("Success! no options...")
	bho, err := ui.QuestionWithDefault("What?!", "Yes")
	fmt.Printf("return bho=%s, err=%v\n", bho, err)
	//num, err := ui.AskForInt("Gimme an int!", 8)
	//fmt.Printf("return num=%d, err=%v\n", num, err)
	ui.Confidential("Confidential message")
	ui.Lifecycle("Lifecycle")
	ui.Warn("warn ! warn !")
	ui.Error("You made an error!")
	ui.Success("Yeee! Done with success!")

	fmt.Println("====== plain and quiet")
	plain := func(ui *clui.Clui) {
		ui.Layout = &clui.PlainLayout{}
	}
	quiet := func(ui *clui.Clui) {
		ui.VerbosityLevel = clui.VerbosityLevelLow
	}
	ui, _ = clui.NewClui(plain, quiet)
	ui.Confidential("Confidential message")
	ui.Lifecycle("Lifecycle")
	ui.Warn("warn ! warn !")
	ui.Error("You made an error!")

}
