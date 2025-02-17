package bone

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

func StrIsAlnum(s string) bool {
	match, _ := regexp.MatchString("^[a-zA-Z0-9]+$", s)
	return match
}

func StrIsFloat(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func StrIsInt(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func StrRemoveSpaces(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, "")
}

func StrWrap(s string, wrapper string) string {
	return fmt.Sprintf("%s%s%s", wrapper, s, wrapper)
}

// Exclude everything except alnum and allowed symbols.
func StrSanitizeAlnumAllowed(name string, allowed []rune) string {
	r := ""
	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			for _, allowedC := range allowed {
				if c == allowedC {
					break
				}
			}
			continue
		}
		r += string(c)
	}
	return r
}

// Helper structure to operate on string.
type Combstring struct {
	Value string
}

func CombstringNew(s string) *Combstring {
	return &Combstring{Value: s}
}

func (cs *Combstring) TrimSpace() *Combstring {
	cs.Value = strings.TrimSpace(cs.Value)
	return cs
}

func (cs *Combstring) ToLower() *Combstring {
	cs.Value = strings.ToLower(cs.Value)
	return cs
}

func (cs *Combstring) ToUpper() *Combstring {
	cs.Value = strings.ToUpper(cs.Value)
	return cs
}

func (cs *Combstring) Replace(old string, new string, count int) *Combstring {
	cs.Value = strings.Replace(cs.Value, old, new, count)
	return cs
}

func (cs *Combstring) ReplaceAll(old string, new string) *Combstring {
	cs.Value = strings.ReplaceAll(cs.Value, old, new)
	return cs
}

func (cs *Combstring) Capitalize() *Combstring {
	if cs.Value == "" {
		return cs
	}
	cs.Value = strings.ToUpper(string(cs.Value[0])) + cs.Value[1:]
	return cs
}

func (cs *Combstring) RemoveSpaces() *Combstring {
	cs.Value = StrRemoveSpaces(cs.Value)
	return cs
}

func (cs *Combstring) SanitizeAlnumAllowed(allowed []rune) *Combstring {
	cs.Value = StrSanitizeAlnumAllowed(cs.Value, allowed)
	return cs
}
