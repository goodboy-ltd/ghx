package proxy

import (
	"slices"
	"testing"

	"github.com/barnuri/ghx/internal/config"
)

func TestNeedsToken(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"no args":             {nil, true},
		"pr list":             {[]string{"pr", "list"}, true},
		"auth status":         {[]string{"auth", "status"}, false},
		"auth login":          {[]string{"auth", "login"}, false},
		"auth alone":          {[]string{"auth"}, false},
		"auth git-credential": {[]string{"auth", "git-credential", "get"}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := NeedsToken(tc.args); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWithProfileReplacesExistingToken(t *testing.T) {
	env := WithProfile([]string{"PATH=/bin", "GH_TOKEN=old"}, config.Profile{User: "me"}, "new")
	if !slices.Equal(env, []string{"PATH=/bin", "GH_TOKEN=new"}) {
		t.Fatalf("got %v", env)
	}
}

func TestWithProfileEnterpriseHost(t *testing.T) {
	env := WithProfile([]string{"GH_HOST=other"}, config.Profile{User: "me", Host: "ghe.example.com"}, "tok")
	if !slices.Equal(env, []string{"GH_ENTERPRISE_TOKEN=tok", "GH_HOST=ghe.example.com"}) {
		t.Fatalf("got %v", env)
	}
}

func TestHasExplicitToken(t *testing.T) {
	if HasExplicitToken([]string{"GH_TOKEN="}) || !HasExplicitToken([]string{"GH_ENTERPRISE_TOKEN=x"}) {
		t.Fatal("unexpected explicit token detection")
	}
}

func TestExplicitTokenVar(t *testing.T) {
	cases := map[string]struct {
		env  []string
		want string
	}{
		"none":              {[]string{"PATH=/bin"}, ""},
		"empty GH_TOKEN":    {[]string{"GH_TOKEN="}, ""},
		"GH_TOKEN":          {[]string{"GH_TOKEN=x"}, "GH_TOKEN"},
		"enterprise":        {[]string{"GH_ENTERPRISE_TOKEN=x"}, "GH_ENTERPRISE_TOKEN"},
		"both":              {[]string{"GH_ENTERPRISE_TOKEN=x", "GH_TOKEN=y"}, "GH_TOKEN"},
		"GITHUB_TOKEN only": {[]string{"GITHUB_TOKEN=x"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ExplicitTokenVar(tc.env); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
