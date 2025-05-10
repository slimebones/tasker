package bone

import (
	"testing"
)

func Test_capitalize_ok(t *testing.T) {
	var cs *Combstring

	cs = CombstringNew("")
	Assert(cs.Capitalize().Value == "")

	cs = CombstringNew("h")
	Assert(cs.Capitalize().Value == "H")

	cs = CombstringNew("he")
	Assert(cs.Capitalize().Value == "He")

	cs = CombstringNew("hello world")
	Assert(cs.Capitalize().Value == "Hello world")
}
