package ext

import (
	"testing"
)

func TestHumanizeSize(t *testing.T) {
	cases := []struct {
		size int
		want string
	}{
		{0, "0 bytes"},
		{1300, "1.27 KB"},
		{1300 * 1024, "1.27 MB"},
		{1300 * 1024 * 1024, "1.27 GB"},
		{1300 * 1024 * 1024 * 1024, "1300.00 GB"},
	}
	for _, tt := range cases {
		got := HumanizeSize(tt.size)
		if got != tt.want {
			t.Errorf("HumanizeSize(%d) = %q; want %q", tt.size, got, tt.want)
		}
	}
}

func TestParseVariables(t *testing.T) {
	for _, tc := range []struct {
		name    string
		inputs  []string
		initial map[string]string
		want    string
	}{
		{name: "plain assignment", inputs: []string{"REPO=registry/default"}, want: "registry/default"},
		{name: "conditional default", inputs: []string{"REPO?=registry/default"}, want: "registry/default"},
		{name: "conditional whitespace", inputs: []string{" REPO ?= registry/default "}, want: "registry/default"},
		{name: "earlier file wins over default", inputs: []string{"REPO=registry/custom", "REPO?=registry/default"}, want: "registry/custom"},
		{name: "first default wins", inputs: []string{"REPO?=registry/first\nREPO?=registry/second"}, want: "registry/first"},
		{name: "plain assignment overrides default", inputs: []string{"REPO?=registry/default", "REPO=registry/custom"}, want: "registry/custom"},
		{name: "plain assignment overrides earlier file", inputs: []string{"REPO=registry/first", "REPO=registry/second"}, want: "registry/second"},
		{name: "defined empty value preserved", initial: map[string]string{"REPO": ""}, inputs: []string{"REPO?=registry/default"}, want: ""},
		{name: "equals in value", inputs: []string{"REPO?=registry/value=extra"}, want: "registry/value=extra"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := tc.initial
			if values == nil {
				values = map[string]string{}
			}
			for _, input := range tc.inputs {
				parseVariables(input, values)
			}
			if got := values["REPO"]; got != tc.want {
				t.Errorf("REPO = %q; want %q", got, tc.want)
			}
			if _, exists := values["REPO?"]; exists {
				t.Error("conditional assignment created REPO? key")
			}
		})
	}
}
