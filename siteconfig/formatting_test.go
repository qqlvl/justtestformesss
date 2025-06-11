package siteconfig

import "testing"

type testCase struct {
	ToFormat string
	Expect   string
}

func TestFormatting(t *testing.T) {
	dictionary := map[string]string{
		"var1": "value1",
		"var2": "value2",
	}

	testCases := []testCase{
		{
			ToFormat: "This is a test string with no vars",
			Expect:   "This is a test string with no vars",
		},
		{
			ToFormat: "This is a test string with one var: ${var1}",
			Expect:   "This is a test string with one var: value1",
		},
		{
			ToFormat: "This is a test string with two vars: ${var1} and ${var2}",
			Expect:   "This is a test string with two vars: value1 and value2",
		},
		{
			ToFormat: "This is a test string with upper var: ${var1:upper}",
			Expect:   "This is a test string with upper var: VALUE1",
		},
		{
			ToFormat: "This is a test string with title var: ${var1:title}",
			Expect:   "This is a test string with title var: Value1",
		},
		{
			ToFormat: "This is a test string with leftpad var: ${var1:leftpad(10)}",
			Expect:   "This is a test string with leftpad var:     value1",
		},
		{
			ToFormat: "This is a test string with rightpad var: ${var1:rightpad(10)}",
			Expect:   "This is a test string with rightpad var: value1    ",
		},
		{
			ToFormat: "This is a test string with title and rightpad var: ${var1:title:rightpad(10)}",
			Expect:   "This is a test string with title and rightpad var: Value1    ",
		},
	}

	for i, tc := range testCases {
		got, err := formatString(tc.ToFormat, dictionary)
		if err != nil {
			t.Errorf("Unexpected error in case %d: %v", i, err)
		}
		if got != tc.Expect {
			t.Errorf("case %d: Expected %s, got %s", i, tc.Expect, got)
		}
	}
}
