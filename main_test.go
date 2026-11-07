package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/barnuri/ghx/internal/config"
	"github.com/barnuri/ghx/internal/resolve"
)

func TestPrintWhichReportsTokenOverride(t *testing.T) {
	matched := resolve.Match{ProfileName: "work", Profile: config.Profile{User: "octocat"}, Source: "/tmp/.ghx"}
	cases := map[string]struct {
		match resolve.Match
		env   []string
		want  string
	}{
		"GH_TOKEN set":            {matched, []string{"GH_TOKEN=x"}, "token:   overridden by GH_TOKEN"},
		"GH_ENTERPRISE_TOKEN set": {matched, []string{"GH_ENTERPRISE_TOKEN=x"}, "token:   overridden by GH_ENTERPRISE_TOKEN"},
		"no token":                {matched, []string{"PATH=/bin"}, ""},
		"no profile":              {resolve.Match{}, []string{"GH_TOKEN=x"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			printWhich(&out, tc.match, "/usr/bin/gh", tc.env)
			got := out.String()
			if tc.want == "" {
				if strings.Contains(got, "token:") {
					t.Fatalf("unexpected token line in %q", got)
				}
				return
			}
			if !strings.Contains(got, "\n"+tc.want) {
				t.Fatalf("output %q lacks %q", got, tc.want)
			}
		})
	}
}
