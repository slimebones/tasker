// Works with "Dogs" - special macros starting with `@`, independent of the language.
// `@` is chosen since it is not confusing with popular programming languages macro: `#`.
// Dog because it looks like a dog! Look @@@@!
//
// The system is flexible and obedient - it won't gawk until finds according tag.
// But it be in explicit verbose mode to warn about incorrect tags.
//
// We have a static list of available macros:
// * ifdef VAR (closeable): includes following block if passed variable is defined
// * close (marked as "@"): closes previous statement
// * todo TEXT
// * copypasta TEXT
// * wip TEXT
//
// A macros is correct only if it is a single owner of the line - this way
// we can omit usages of `@` that are out of context. The macro line can be prefixed
// with comment sequence, which is one of these (indentations before starting sequence are allowed):
// * `//`
// * `#`
// * `;`
// * `--`
// * `<!--`
// Note that strict amount of spaces is required: `// @COMMAND ...` will be
// accepted and `//     @COMMAND ...` will not. This will ensure strict
// semantics are followed.
//
// All macros support being invoked without arguments. For example, `@ifdef`
// invoked without args will result of following block to be always defined.
package dog

import (
	"io"
	"os"
	"regexp"
	"strings"
	"tasker/internal/bone"
)

const (
	Ok    = 0
	Error = 1
)

type ProcessArgs struct {
	TargetPath string
	Vars       map[string]string
}

// Executes file tags and returns resulting string file body.
//
// Note that we work only with text files.
func ProcessFile(p string, argVars map[string]string) (*string, bool) {
	f, e := os.Open(p)
	if e != nil {
		return nil, false
	}
	defer f.Close()

	bodyBytes, e := io.ReadAll(f)
	if e != nil {
		return nil, false
	}
	body := string(bodyBytes)
	return Process(body, argVars)
}

func Process(body string, argVars map[string]string) (*string, bool) {
	vars := make(map[string]string, len(argVars))
	for k, v := range argVars {
		vars[k] = v
	}

	body = strings.ReplaceAll(body, "\r\n", "\n")
	bodyPtr := &body
	lines := strings.Split(body, "\n")

	tokens, ok := lexer(lines)
	if !ok {
		return nil, false
	}
	executor(tokens, vars, bodyPtr)

	return bodyPtr, true
}

type MacroControl int

const (
	Control_None MacroControl = iota
	Control_Opener
	Control_Closer
)

type Macro struct {
	Command string
	Control MacroControl
	Args    string
	Text    string
	Col     int
}

var OPENER_COMMENTS = []string{
	"@",
	"// @",
	"# @",
	"; @",
	"-- @",
	"<!-- @",
}
var OPENER_MACROS = []string{
	"ifdef",
}

// We don't even consider failed lines even if they started looking like macros.
func lexer(lines []string) ([]*Macro, bool) {
	blockOpened := false
	macros := []*Macro{}

	for col, line := range lines {
		col += 1
		// Allow indentation.
		line = strings.TrimSpace(line)
		for _, openerComment := range OPENER_COMMENTS {
			if strings.HasPrefix(line, openerComment) {
				macro, ok := parseMacroLine(col, line, openerComment)
				if ok {
					ok = defineMacroConrol(macro, blockOpened)
					if ok {
						macros = append(macros, macro)
						blockOpened = macro.Control == Control_Opener
					}
				}
				break
			}
		}
	}

	return macros, true
}

// Find out to which control group macro can be assigned.
func defineMacroConrol(macro *Macro, currentlyOpened bool) bool {
	macro.Control = Control_None
	if macro.Command == "close" {
		if !currentlyOpened {
			// we just skip uncorrectly placed close macro
			// other macros remain untouched
			return false
		}
		macro.Control = Control_Closer
		return true
	}

	for _, openerMacro := range OPENER_MACROS {
		if macro.Command == openerMacro {
			macro.Control = Control_Opener
			break
		}
	}

	if macro.Control == Control_Opener && currentlyOpened {
		return false
	}

	return true
}

// Parse macro out of line.
func parseMacroLine(col int, line string, opener string) (*Macro, bool) {
	macro := &Macro{
		Text: line,
		Col:  col,
	}
	withoutOpener, _ := strings.CutPrefix(line, opener)
	if len(withoutOpener) == 0 {
		macro.Command = "close"
		macro.Args = ""
		return macro, true
	}
	parts := strings.Split(withoutOpener, " ")
	if len(parts) == 0 {
		return macro, false
	}
	macro.Command = parts[0]
	// can't use directly
	if macro.Command == "close" {
		return nil, false
	}
	macro.Args = strings.Join(parts[1:], "")
	return macro, true
}

var VAR_REGEX = regexp.MustCompile(`[A-Z0-9_]*`)

// Execute list of macros to modify body.
func executor(macros []*Macro, vars map[string]string, body *string) bool {
	// for our current set of macros we only delete
	deletionCols := []int{}
	ignoreNextCloser := false
	deleteUntilNextCloser := false
	lastOpenerCol := -1

	for _, macro := range macros {
		if macro.Command == "ifdef" {
			args := strings.TrimSpace(macro.Args)
			valid := VAR_REGEX.MatchString(args)
			if !valid {
				ignoreNextCloser = true
				continue
			}
			_, found := vars[args]
			if !found {
				deleteUntilNextCloser = true
				lastOpenerCol = macro.Col
			}
		}
		if macro.Control == Control_Closer {
			if ignoreNextCloser {
				ignoreNextCloser = false
				continue
			}
			if deleteUntilNextCloser {
				deleteUntilNextCloser = false
				deletionCols = append(
					deletionCols,
					getColsBetween(lastOpenerCol, macro.Col)...,
				)
				lastOpenerCol = -1
			}
		}
		deletionCols = append(deletionCols, macro.Col)
	}

	deleteCols(deletionCols, body)
	return true
}

// Between, but not including cols itself.
func getColsBetween(openerCol int, closerCol int) []int {
	cols := []int{}
	for col := openerCol + 1; col < closerCol; col++ {
		cols = append(cols, col)
	}
	return cols
}

// Cols must be ASC sorted.
func deleteCols(cols []int, body *string) {
	newBody := ""
	for col, line := range strings.Split(*body, "\n") {
		col += 1
		skipped := false
		for _, deletionCol := range cols {
			if col == deletionCol {
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}
		newBody += "\n" + line
	}
	newBody, found := strings.CutPrefix(newBody, "\n")
	bone.Assert(found)
	*body = newBody
}
