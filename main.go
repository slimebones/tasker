package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"tasker/lib/bone"
	"tasker/lib/db"
)

func main() {
	bone.Init(bone.Init_Args{
		Company_Name: "slimebones",
		App_Name:     "tasker",
	})
	e := db.Init()
	if e > 0 {
		panic("Failed to initialize db")
	}
	defer db.Deinit()
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
		if input == "q" {
			return
		}
		process_input(input)
	}
}

type handler func(ctx *Command_Context) int

type Command_Context struct {
	Raw_input    string
	Command_name string
	Args         []string
}

const (
	Ok = iota
	Error
	Input_Error
)

const (
	SOMETIME_LATER_PRIORITY = iota
	THIS_WEEK_PRIORITY
	TODAY_PRIORITY
)

var current_project_id = 0

var date_regex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
var time_regex = regexp.MustCompile(`\s\d{2}:\d{2}`)

func add_task(ctx *Command_Context) int {
	if len(ctx.Args) == 0 {
		bone.Log_Error("Enter task title")
		return Input_Error
	}
	// Purified title
	title := ""
	var schedule *string = nil
	priority := 0
	for _, a := range ctx.Args {
		if date_regex.MatchString(a) {
			if schedule != nil {
				bone.Log_Error("Multiple date defined")
				return Input_Error
			}
			schedule = &a
			continue
		}
		if time_regex.MatchString(a) {
			if schedule == nil {
				bone.Log_Error("Time precedes date")
				return Input_Error
			}
			inter := *schedule + a
			schedule = &inter
			continue
		}

		if title == "p1" {
			priority = SOMETIME_LATER_PRIORITY
			continue
		}
		if title == "p2" {
			priority = THIS_WEEK_PRIORITY
			continue
		}
		if title == "p3" {
			priority = TODAY_PRIORITY
			continue
		}
		if priority == 0 {
			if schedule != nil {
				bone.Log_Error("Priority and schedule are set simultaneously")
				return Input_Error
			}
			// We default to sometime later priority.
			priority = 1
			continue
		}

		title += a + " "
	}
	title = strings.TrimSpace(title)

	tx := db.Begin()
	r, er := tx.Exec(
		"INSERT INTO task (title, completion_priority, schedule, project_id) VALUES ($1, $2, $3, $4)",
		title,
		priority,
		schedule,
		current_project_id,
	)
	if er != nil {
		bone.Log_Error("During task insertion, an error occured: %s", er)
		return Error
	}
	last_id, er := r.LastInsertId()
	if er != nil {
		bone.Log_Error("During last insert id retrieve, an error occured: %s", er)
		return Error
	}

	er = tx.Commit()
	if er != nil {
		bone.Log_Error("During commit, an error occured: %s", er)
		return Error
	}

	bone.Log("Created task #%d", last_id)

	return Ok
}

func show_tasks(ctx *Command_Context) int {
	// tx := db.Begin()

	return Ok
}

var COMMANDS = map[string]handler{
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
		bone.Log_Error("While calling a command `%s`, an error occured: %s", command_name, bone.Tr_Code(e))
	}
}
