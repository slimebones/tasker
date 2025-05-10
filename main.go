package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"tasker/internal/bone"
	"tasker/internal/common"
	"tasker/internal/db"

	"github.com/jmoiron/sqlx"
)

type handler func(ctx *Command_Context) int

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

func (t *Task) Get_Priority_Mark() string {
	switch t.Priority {
	case 1:
		return "🟡"
	case 2:
		return "🔴"
	// Everything unusual is considered as active.
	default:
		return "🟢"
	}
}

func (t *Task) Get_Completion_Mark() string {
	switch t.State {
	case 1:
		return "\033[32m+\033[0m"
	case 2:
		return "\033[31m-\033[0m"
	default:
		// Everything unusual is considered as active.
		return "\033[35m.\033[0m"
	}
}

var COMMANDS = map[string]handler{
	"s": show,
	"a": add_task,
	"u": update_task,
	"w": switch_project,
	"p": add_project,
	"i": info,
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

func (ctx *Command_Context) Has_Arg_Index(arg string) (bool, int) {
	for i, a := range ctx.Args {
		if a == arg {
			return true, i
		}
	}
	return false, -1
}

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

var hooks = []any{}

func set_hooks[T any](items []T) {
	clear_hooks()
	for _, i := range items {
		hooks = append(hooks, i)
	}
}

func clear_hooks() {
	hooks = []any{}
}

// Show statistics of the current project.
func info(ctx *Command_Context) int {
	return common.OK
}

func add_project(ctx *Command_Context) int {
	tx := db.Begin()
	defer tx.Rollback()

	_, er := tx.Exec("INSERT INTO project (title) VALUES ($1)", ctx.Args[0])
	if er != nil {
		bone.Log_Error("During project add, cannot insert project with title '%s', the error is: %s", ctx.Args[0], er.Error())
		return common.INSERT_ERROR
	}

	er = tx.Commit()
	if er != nil {
		return common.COMMIT_ERROR
	}

	bone.Log("Added project '%s'.", ctx.Args[0])

	return common.OK
}

// Switch the current active project.
//
// Args:
//   - 1 (default="main"): Name of the project to switch to.
func switch_project(ctx *Command_Context) int {
	project_name := "main"
	if len(ctx.Args) > 0 {
		project_name = ctx.Args[0]
	}

	tx := db.Begin()
	defer tx.Rollback()

	type Temp_Project struct {
		Id int `db:"id"`
	}
	var project Temp_Project
	er := tx.Get(&project, "SELECT id FROM project WHERE title = $1", project_name)
	if er != nil {
		bone.Log_Error("During searching for project '%s', an error occurred: %s.", project_name, er.Error())
		return common.ERROR
	}

	current_project_id = project.Id
	current_project_name = project_name

	return common.OK
}

func find(ctx *Command_Context) int {
	return common.OK
}

var prompted = false
var prompted_callback func(answer bool) int = nil

func answer_prompt(answer bool) {
	if !prompted {
		bone.Log_Error("Inactive prompt")
		return
	}
	prompted = false
	e := prompted_callback(answer)
	if e != common.OK {
		bone.Log_Error("During prompted callback, an error #%d occured", e)
	}
	prompted_callback = nil
}

func prompt(text string, callback func(answer bool) int) {
	if prompted {
		bone.Log_Error("Already prompted")
		return
	}
	prompted = true
	prompted_callback = callback
	fmt.Println(text + " [Y/N]")
}

func escape_quotes(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
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
//   - `-n TITLE`: set title
//   - `-np TEXT`: prepend to title
//   - `-na TEXT`: append to title
func update_task(ctx *Command_Context) int {
	var er error

	if len(ctx.Args) == 0 {
		return common.INPUT_ERROR
	}

	tx := db.Begin()
	defer tx.Rollback()

	task_ids := []int{}
	parts := strings.Split(ctx.Args[0], "+")
	for _, p := range parts {
		hook_number, er := strconv.Atoi(p)
		if er != nil {
			bone.Log_Error("Cannot convert order number `%d` to integer", hook_number)
			return common.INPUT_ERROR
		}
		if hook_number-1 >= len(hooks) {
			bone.Log_Error("Cannot find task with hook number `%d`", hook_number)
			return common.INPUT_ERROR
		}
		task, ok := hooks[hook_number-1].(*Task)
		if !ok {
			bone.Log_Error("Hook #%d is not a Task", hook_number)
			return common.HOOK_TYPE_ERROR
		}
		task_ids = append(task_ids, task.Id)
	}

	where_query, where_args, er := sqlx.In("id in (?)", task_ids)
	if er != nil {
		bone.Log_Error("During query building, an error occured: %s", er)
		return common.DELETE_ERROR
	}
	where_query = tx.Rebind(where_query)

	set_query := fmt.Sprintf("SET state = 1, last_completed_sec = %d", bone.Utc())

	if ctx.Has_Arg("-d") {
		var delete_tasks = func(answer bool) int {
			if answer {
				tx := db.Begin()
				defer tx.Rollback()
				delete_query := fmt.Sprintf("DELETE FROM task WHERE %s", where_query)
				_, er = tx.Exec(delete_query, where_args...)
				if er != nil {
					return common.DELETE_ERROR
				}

				er = tx.Commit()
				if er != nil {
					return common.COMMIT_ERROR
				}

				fmt.Printf("Deleted\n")
			}
			return common.OK
		}
		var task_label = "task"
		if len(parts) > 1 {
			task_label = "tasks"
		}
		prompt(fmt.Sprintf("Delete %s %s?", task_label, strings.Join(parts, ",")), delete_tasks)
		return common.OK
	}
	if ctx.Has_Arg("-r") {
		set_query = fmt.Sprintf("SET state = 2, last_rejected_sec = %d", bone.Utc())
	}
	if ctx.Has_Arg("-m") {
		bone.Log_Error("Move is not supported yet")
		return common.ERROR
	}
	if has, index := ctx.Has_Arg_Index("-n"); has {
		if index+1 >= len(ctx.Args) {
			bone.Log_Error("-n parameter missing title")
			return common.INPUT_ERROR
		}

		title := ""
		for _, a := range ctx.Args[index+1:] {
			// This is final parameter - we don't stop here for any other commands
			title += a + " "
		}
		title, _ = strings.CutSuffix(title, " ")

		set_query = fmt.Sprintf("SET title = '%s'", escape_quotes(title))
	}
	if has, index := ctx.Has_Arg_Index("-na"); has {
		if index+1 >= len(ctx.Args) {
			bone.Log_Error("-na parameter missing text")
			return common.INPUT_ERROR
		}

		// Add space prefix as we want it by default
		title := " "
		for _, a := range ctx.Args[index+1:] {
			// This is final parameter - we don't stop here for any other commands
			title += a + " "
		}
		title, _ = strings.CutSuffix(title, " ")

		set_query = fmt.Sprintf("SET title = title || '%s'", escape_quotes(title))
	}
	if has, index := ctx.Has_Arg_Index("-np"); has {
		if index+1 >= len(ctx.Args) {
			bone.Log_Error("-np parameter missing text")
			return common.INPUT_ERROR
		}

		title := ""
		for _, a := range ctx.Args[index+1:] {
			// This is final parameter - we don't stop here for any other commands
			title += a + " "
		}
		// Do not cut space suffix as we do want it by default

		set_query = fmt.Sprintf("SET title = '%s' || title", escape_quotes(title))
	}

	query := fmt.Sprintf("UPDATE task %s WHERE %s", set_query, where_query)
	_, er = tx.Exec(
		query,
		where_args...,
	)
	if er != nil {
		bone.Log_Error("During update, an error occured: %s", er)
		return common.UPDATE_ERROR
	}

	er = tx.Commit()
	if er != nil {
		return common.COMMIT_ERROR
	}

	fmt.Printf("Updated\n")
	return common.OK
}

func add_task(ctx *Command_Context) int {
	if len(ctx.Args) == 0 {
		bone.Log_Error("Enter task title")
		return common.INPUT_ERROR
	}
	// Purified title
	title := ""
	var schedule *string = nil
	priority := -1
	for _, arg := range ctx.Args {
		if date_regex.MatchString(arg) {
			if schedule != nil {
				bone.Log_Error("Multiple date defined")
				return common.INPUT_ERROR
			}
			schedule = &arg
			continue
		}
		if time_regex.MatchString(arg) {
			if schedule == nil {
				bone.Log_Error("Time precedes date")
				return common.INPUT_ERROR
			}
			inter := *schedule + arg
			schedule = &inter
			continue
		}

		if arg == "p3" {
			priority = SOMETIME_LATER_PRIORITY
			continue
		}
		if arg == "p2" {
			priority = THIS_WEEK_PRIORITY
			continue
		}
		if arg == "p1" {
			priority = TODAY_PRIORITY
			continue
		}

		title += arg + " "
	}
	if priority != -1 && schedule != nil {
		bone.Log_Error("Priority and schedule are set simultaneously")
		return common.INPUT_ERROR
	} else if priority == -1 {
		// Default
		priority = SOMETIME_LATER_PRIORITY
	}
	title = strings.TrimSpace(title)

	tx := db.Begin()
	defer tx.Rollback()
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
		return common.ERROR
	}
	_, er = r.LastInsertId()
	if er != nil {
		bone.Log_Error("During last insert id retrieve, an error occured: %s", er)
		return common.ERROR
	}

	er = tx.Commit()
	if er != nil {
		bone.Log_Error("During commit, an error occured: %s", er)
		return common.ERROR
	}

	fmt.Print("Created\n")

	return common.OK
}

// Show tasks from the current active project. By default only active tasks
// are shown.
//
// Default chronological order: oldest first.
//
// Args:
//   - `-p`: show projects instead of tasks
//   - `-reverse`: reverse order
//   - `-a`: show all
//   - `-c`: show only completed
//   - `-r`: show only rejected
//   - `-screated`: show creation times
//   - `-scompleted`: show completion times
//   - `-srejected`: show rejection times
//   - `-ocompleted`: order by completion time, integrates with `-reverse`
//   - `-orejected`: order by rejection time, integrates with `-reverse`
func show(ctx *Command_Context) int {
	project_show := ctx.Has_Arg("-p")

	query := ""
	where_query := ""
	order_query := ""

	if !project_show {
		where_query = "WHERE state = 0"
		if ctx.Has_Arg("-c") {
			where_query = "WHERE state = 1"
		}
		if ctx.Has_Arg("-r") {
			where_query = "WHERE state = 2"
		}
		where_query += fmt.Sprintf(" AND project_id = %d", current_project_id)

		order_query = "ORDER BY created_sec ASC"
		if ctx.Has_Arg("-reverse") {
			order_query = "ORDER BY created_sec DESC"
		}

		if ctx.Has_Arg("-ocompleted") {
			order_query = "ORDER BY last_completed_sec ASC"
			if ctx.Has_Arg("-reverse") {
				order_query = "ORDER BY last_completed_sec DESC"
			}
		}
		if ctx.Has_Arg("-orejected") {
			order_query = "ORDER BY last_rejected_sec ASC"
			if ctx.Has_Arg("-reverse") {
				order_query = "ORDER BY last_rejected_sec DESC"
			}
		}

		if ctx.Has_Arg("-a") {
			where_query = ""
			// Show active first, completed second, rejected last
			order_query = "ORDER BY state DESC, created_sec ASC"
			if ctx.Has_Arg("-reverse") {
				order_query = "ORDER BY state ASC, created_sec DESC"
			}
		}
	}

	query = "SELECT * FROM task"
	if project_show {
		query = "SELECT * from project"
	}
	query += " %s %s"
	query = fmt.Sprintf(query, where_query, order_query)
	tx := db.Begin()
	defer tx.Rollback()

	if !project_show {
		targets := []*Task{}
		er := tx.Select(&targets, query)
		if er != nil {
			bone.Log_Error("During task selection, an error occured: %s", er)
			return common.ERROR
		}
		set_hooks(targets)
		if len(hooks) == 0 {
			fmt.Print("No tasks\n")
		}
		for i, h := range hooks {
			t, ok := h.(*Task)
			if !ok {
				bone.Log_Error("Hook #%d is not a task", i+1)
				return common.HOOK_TYPE_ERROR
			}
			if ctx.Has_Arg("-screated") {
				fmt.Printf("|%d| %s |%s| %s\n", i+1, t.Get_Completion_Mark(), convert_sec_to_str(t.Created_Sec), t.Title)
			} else if ctx.Has_Arg("-scompleted") {
				fmt.Printf("|%d| %s |%s| %s\n", i+1, t.Get_Completion_Mark(), convert_sec_to_str(t.Last_Completed_Sec), t.Title)
			} else if ctx.Has_Arg("-srejected") {
				fmt.Printf("|%d| %s |%s| %s\n", i+1, t.Get_Completion_Mark(), convert_sec_to_str(t.Last_Rejected_Sec), t.Title)
			} else {
				fmt.Printf("|%d| %s %s\n", i+1, t.Get_Completion_Mark(), t.Title)
			}
		}
	} else {
		targets := []*Project{}
		er := tx.Select(&targets, query)
		if er != nil {
			bone.Log_Error("During project selection, an error occured: %s", er)
			return common.ERROR
		}
		set_hooks(targets)
		if len(hooks) == 0 {
			// This shouldn't be possible.
			fmt.Print("No projects?\n")
		}
		for i, h := range hooks {
			t, ok := h.(*Project)
			if !ok {
				bone.Log_Error("Hook #%d is not a project.", i+1)
				return common.HOOK_TYPE_ERROR
			}
			fmt.Printf("|%d| %s\n", i+1, t.Title)
		}
	}

	return common.OK
}

func convert_sec_to_str(sec int) string {
	return bone.Date_Sec(sec, "2006-01-02 15:04")
}

func process_input(input string) {
	// Quoted strings are not yet supported - they will be separated as everything else.
	input_parts := strings.Fields(input)
	if len(input_parts) == 0 {
		return
	}

	if prompted {
		var answer bool
		switch input {
		case "y":
			answer = true
		case "n":
			answer = false
		case "Y":
			answer = true
		case "N":
			answer = false
		default:
			bone.Log("Type answer 'Y' or 'N'")
			return
		}
		answer_prompt(answer)
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
		return
	}
}

func main() {
	bone.Init("tasker")
	e := db.Init()
	if e > 0 {
		panic("Failed to initialize db")
	}
	defer db.Deinit()

	// Execute one-shot command
	if len(os.Args) > 2 && os.Args[1] == "--" {
		input := strings.Join(os.Args[2:], " ")
		input = strings.TrimSpace(input)
		if input == "q" {
			return
		}
		process_input(input)
		return
	}

	console_reader := bufio.NewReader(os.Stdin)

	// Main loop is blocking on input, other background tasks are goroutines.
	for {
		var final_sign = ">"
		if prompted {
			final_sign = "?"
		}
		fmt.Printf("\033[33m(%s)\033[0m\033[35m%s\033[0m ", current_project_name, final_sign)
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
