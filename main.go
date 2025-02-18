package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"tasker/lib/bone"
	"tasker/lib/db"
)

func parallel_init() {
	e := db.Init()
	if e > 0 {
		panic("Failed to initialize db")
	}
}

func main() {
	bone.Init(bone.Init_Args{
		Company_Name: "slimebones",
		App_Name:     "tasker",
	})
	go parallel_init()
	console_reader := bufio.NewReader(os.Stdin)

	// Main loop is blocking on input, other background tasks are goroutines.
	for {
		fmt.Print("> ")
		input, er := console_reader.ReadString('\n')
		if er != nil {
			if er.Error() != "EOF" {
				bone.Log_Error("Unexpected error occured while reading console: %s", er)
			}
			return
		}
		input = strings.TrimSpace(input)
		process_input(input)
	}
}

type handler func(ctx *Command_Context) int

type Command_Context struct {
	Raw_input    string
	Command_name string
	Args         []string
}

func add_task(ctx *Command_Context) int {
	return 0
}

func show_tasks(ctx *Command_Context) int {
	return 0
}

func quit(_ *Command_Context) int {
	os.Exit(0)
	return 0
}

var COMMANDS = map[string]handler{
	"q": quit,
	"s": show_tasks,
	"a": add_task,
}

func process_input(input string) {
	// Quoted strings are not yet supported - they will be separated as everything else.
	input_parts := strings.Fields(input)
	if len(input_parts) == 0 {
		return
	}

	command_name := input_parts[0]

	cmd, ok := COMMANDS[command_name]
	if !ok {
		bone.Log_Error("Unrecognized command: " + input)
		return
	}

	args := []string{}
	if len(input_parts) > 1 {
		args = input_parts[1:]
	}
	ctx := Command_Context{
		Raw_input:    input,
		Command_name: command_name,
		Args:         args,
	}

	e := cmd(&ctx)
	if e > 0 {
		bone.Log_Error("While calling a command `%s`, an error occured: %s", bone.Tr_Code(e))
	}
}
