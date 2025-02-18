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

type handler func(input string) error

func add_task(input string) error {
	return nil
}

func quit(_ string) error {
	os.Exit(0)
	return nil
}

var COMMANDS = map[string]handler{
	"q": quit,
	"a": add_task,
}

func process_input(input string) {
	cmd, ok := COMMANDS[input]
	if !ok {
		bone.Log_Error("Unrecognized command: " + input)
		return
	}

	e := cmd(input)
	if e != nil {
		bone.Log_Error(e.Error())
	}
}
