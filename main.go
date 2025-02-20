package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"tasker/lib/bone"
	"tasker/lib/db"

	"github.com/jmoiron/sqlx"
)

type handler func(ctx *Command_Context) int

var COMMANDS = map[string]handler{
	"s":    show_tasks,
	"a":    add_task,
	"u":    update_task,
	"w":    switch_project,
	"stat": stat,
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
	COMMIT_ERROR
	UPDATE_ERROR
	DELETE_ERROR
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

func clear_hooks() {
	hook_projects = []*Project{}
	hook_tasks = []*Task{}
}

// Show statistics of the current project.
func stat(ctx *Command_Context) int {
	return OK
}

func switch_project(ctx *Command_Context) int {
	return OK
}

// Change task out of last rendered tasks by order number.
//
// Default behaviour: mark as completed.
//
// Args:
//   - `%d`: hook number, can be chained like `%d+%d+...` to affect multiple tasks
//   - `-r`: mark as rejected
//   - `-d`: delete forever
//   - `-m PROJECT_NAME`: move to another project
func update_task(ctx *Command_Context) int {
	var er error

	if len(ctx.Args) == 0 {
		return INPUT_ERROR
	}

	task_ids := []int{}
	parts := strings.Split(ctx.Args[0], "+")
	for _, p := range parts {
		hook_number, er := strconv.Atoi(p)
		if er != nil {
			bone.Log_Error("Cannot convert order number `%d` to integer", hook_number)
			return INPUT_ERROR
		}
		if hook_number-1 >= len(hook_tasks) {
			bone.Log_Error("Cannot find task with hook number `%d`", hook_number)
			return INPUT_ERROR
		}
		task_ids = append(task_ids, hook_tasks[hook_number-1].Id)
	}
	tx := db.Begin()
	defer tx.Rollback()

	where_query, args, er := sqlx.In("id in (?)", task_ids)
	if er != nil {
		bone.Log_Error("During query building, an error occured: %s", er)
		return DELETE_ERROR
	}
	where_query = tx.Rebind(where_query)

	set_query := "SET state = 1"

	if ctx.Has_Arg("-d") {
		_, er = tx.Exec(fmt.Sprintf("DELETE FROM task WHERE %s", where_query), args...)
		if er != nil {
			return DELETE_ERROR
		}

		er = tx.Commit()
		if er != nil {
			return COMMIT_ERROR
		}

		fmt.Printf("Deleted\n")
		clear_hooks()
		return OK
	}
	if ctx.Has_Arg("-r") {
		set_query = "SET state = 2"
	}
	if ctx.Has_Arg("-m") {
		bone.Log_Error("Move is not supported yet")
		return ERROR
	}

	_, er = tx.Exec(
		fmt.Sprintf("UPDATE task %s WHERE %s", set_query, where_query),
		task_ids[0],
	)
	if er != nil {
		return UPDATE_ERROR
	}

	er = tx.Commit()
	if er != nil {
		return COMMIT_ERROR
	}

	fmt.Printf("Updated\n")
	clear_hooks()
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

type Project struct {
	Id    int    `db:"id"`
	Title string `db:"title"`
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

var hook_projects = []*Project{}
var hook_tasks = []*Task{}

// Show tasks from the current active project. By default only active tasks
// are shown.
//
// Default chronological order: most recent first.
//
// Args:
//   - `-reverse`: reverse order
//   - `-a`: show all
//   - `-c`: show only completed
//   - `-r`: show only rejected
func show_tasks(ctx *Command_Context) int {
	clear_hooks()
	tx := db.Begin()
	defer tx.Rollback()

	where_query := "WHERE state = 0"
	if ctx.Has_Arg("-c") {
		where_query = "WHERE state = 1"
	}
	if ctx.Has_Arg("-r") {
		where_query = "WHERE state = 2"
	}

	order_query := "ORDER BY created_sec DESC"
	if ctx.Has_Arg("-reverse") {
		order_query = "ORDER BY created_sec ASC"
	}
	if ctx.Has_Arg("-a") {
		where_query = ""
		// Show active first, completed second, rejected last
		order_query = "ORDER BY state ASC, created_sec DESC"
		if ctx.Has_Arg("-reverse") {
			order_query = "ORDER BY state DESC, created_sec ASC"
		}
	}

	query := fmt.Sprintf("SELECT * FROM task %s %s", where_query, order_query)

	er := tx.Select(&hook_tasks, query)
	if er != nil {
		bone.Log_Error("During task selection, an error occured: %s", er)
		return ERROR
	}
	if len(hook_tasks) == 0 {
		fmt.Print("No tasks\n")
	}
	for i, t := range hook_tasks {
		fmt.Printf("|%d| %s %s\n", i+1, t.Get_Completion_Mark(), t.Title)
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
		fmt.Printf("\033[33m(%s)\033[0m\033[35m>\033[0m ", current_project_name)
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
