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

type handler func(ctx *Command_Context) int

var COMMANDS = map[string]handler{
	"s": show_tasks,
	"a": add_task,
	"w": switch_project,
}

type Command_Context struct {
	Raw_Input    string
	Command_Name string
	Args         []string
}

func (ctx *Command_Context) Has_Arg(arg string) bool {
	for _, a := range ctx.Args {
		if a == arg {
			return true
		}
	}
	return false
}

const (
	OK = iota
	ERROR
	INPUT_ERROR
)

const (
	SOMETIME_LATER_PRIORITY = iota
	THIS_WEEK_PRIORITY
	TODAY_PRIORITY
)

const (
	ACTIVE = iota
	COMPLETED
	REJECTED
)

var current_project_id = 1
var current_project_name = "main"

var date_regex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
var time_regex = regexp.MustCompile(`\s\d{2}:\d{2}`)

func switch_project(ctx *Command_Context) int {
	return OK
}

func add_task(ctx *Command_Context) int {
	if len(ctx.Args) == 0 {
		bone.Log_Error("Enter task title")
		return INPUT_ERROR
	}
	// Purified title
	title := ""
	var schedule *string = nil
	priority := -1
	for _, a := range ctx.Args {
		if date_regex.MatchString(a) {
			if schedule != nil {
				bone.Log_Error("Multiple date defined")
				return INPUT_ERROR
			}
			schedule = &a
			continue
		}
		if time_regex.MatchString(a) {
			if schedule == nil {
				bone.Log_Error("Time precedes date")
				return INPUT_ERROR
			}
			inter := *schedule + a
			schedule = &inter
			continue
		}

		if title == "p1" {
			priority = SOMETIME_LATER_PRIORITY
		}
		if title == "p2" {
			priority = THIS_WEEK_PRIORITY
		}
		if title == "p3" {
			priority = TODAY_PRIORITY
		}
		if priority != -1 {
			if schedule != nil {
				bone.Log_Error("Priority and schedule are set simultaneously")
				return INPUT_ERROR
			}
			// Clearly set, exclude from final title
			continue
		} else {
			// Default
			priority = SOMETIME_LATER_PRIORITY
		}

		title += a + " "
	}
	title = strings.TrimSpace(title)

	tx := db.Begin()
	r, er := tx.Exec(
		"INSERT INTO task (title, created_sec, completion_priority, schedule, project_id) VALUES ($1, $2, $3, $4, $5)",
		title,
		bone.Utc(),
		priority,
		schedule,
		current_project_id,
	)
	if er != nil {
		bone.Log_Error("During task insertion, an error occured: %s", er)
		return ERROR
	}
	_, er = r.LastInsertId()
	if er != nil {
		bone.Log_Error("During last insert id retrieve, an error occured: %s", er)
		return ERROR
	}

	er = tx.Commit()
	if er != nil {
		bone.Log_Error("During commit, an error occured: %s", er)
		return ERROR
	}

	fmt.Print("Created\n")

	return OK
}

type Task struct {
	Id                 int     `db:"id"`
	Title              string  `db:"title"`
	State              int     `db:"state"`
	Created_Sec        int     `db:"created_sec"`
	Last_Completed_Sec int     `db:"last_completed_sec"`
	Last_Rejected_Sec  int     `db:"last_rejected_sec"`
	Priority           int     `db:"completion_priority"`
	Schedule           *string `db:"schedule"`
	Project_Id         int     `db:"project_id"`
}

func (t *Task) Get_Completion_Mark() string {
	switch t.State {
	case 1:
		return "+"
	case 2:
		return "-"
	// Everything unusual is considered as active.
	default:
		return "."
	}
}

// Show tasks from the current active project. By default only active tasks
// are shown.
//
// Default chronological order: most recent first.
//
// Args:
//   - `-reverse`: reverse chronological order
//   - `-completed`: show only completed
//   - `-rejected`: show only rejected
//   - `-stat`: show statistics
func show_tasks(ctx *Command_Context) int {
	tx := db.Begin()
	defer tx.Rollback()
	tasks := []Task{}
	er := tx.Select(&tasks, "SELECT * FROM task WHERE state = 0 ORDER BY created_sec DESC")
	if er != nil {
		bone.Log_Error("During task selection, an error occured: %s", er)
		return ERROR
	}
	if len(tasks) == 0 {
		fmt.Print("No tasks\n")
	}
	for _, t := range tasks {
		fmt.Printf("%s %s\n", t.Get_Completion_Mark(), t.Title)
	}
	return OK
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
		Raw_Input:    input,
		Command_Name: command_name,
		Args:         args,
	}

	e := cmd(&ctx)
	if e > 0 {
		bone.Log_Error("While calling a command `%s`, an error occured: %s", command_name, bone.Tr_Code(e))
	}
}

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
		fmt.Printf("(%s)> ", current_project_name)
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
