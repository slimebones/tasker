package dog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_with_var_ok(t *testing.T) {
	var expectedBody string
	var body *string
	var ok bool

	expectedBody = "One donut please.\nHave a nice day!"
	body, ok = ProcessFile("test_01.txt", map[string]string{
		"FRIENDLY": "",
	})
	assert.True(t, ok)
	assert.Equal(t, expectedBody, *body)
}

func Test_without_var_ok(t *testing.T) {
	var expectedBody string
	var body *string
	var ok bool

	expectedBody = "One donut please."
	body, ok = ProcessFile("test_01.txt", map[string]string{})
	assert.True(t, ok)
	assert.Equal(t, expectedBody, *body)
}
