package dog

import (
	"tasker/lib/bone"
	"testing"
)

func Test_with_var_ok(t *testing.T) {
	var expectedBody string
	var body *string
	var ok bool

	expectedBody = "One donut please.\nHave a nice day!"
	body, ok = ProcessFile("res/test_01.txt", map[string]string{
		"FRIENDLY": "",
	})
	bone.Assert(ok)
	bone.Assert(expectedBody == *body)
}

func Test_without_var_ok(t *testing.T) {
	var expectedBody string
	var body *string
	var ok bool

	expectedBody = "One donut please."
	body, ok = ProcessFile("res/test_01.txt", map[string]string{})
	bone.Assert(ok)
	bone.Assert(expectedBody == *body)
}
