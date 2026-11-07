package update

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "1.2.3", "1.2.3", 0},
		{"v prefix equal", "v1.2.3", "1.2.3", 0},
		{"v prefix both", "v1.2.3", "v1.2.4", -1},
		{"patch ahead", "1.2.4", "1.2.3", 1},
		{"patch behind", "1.2.3", "1.2.4", -1},
		{"minor ahead", "1.3.0", "1.2.9", 1},
		{"minor behind", "1.2.9", "1.3.0", -1},
		{"major ahead", "2.0.0", "1.9.9", 1},
		{"major behind", "0.1.9", "1.0.0", -1},
		{"numeric not lexical", "0.1.10", "0.1.9", 1},
		{"prerelease below release", "1.0.0-rc.1", "1.0.0", -1},
		{"release above prerelease", "1.0.0", "1.0.0-rc.1", 1},
		{"prerelease above older release", "1.0.1-rc.1", "1.0.0", 1},
		{"prereleases lexical", "1.0.0-rc.2", "1.0.0-rc.1", 1},
		{"prereleases equal", "1.0.0-beta", "1.0.0-beta", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := compareVersions(tc.a, tc.b)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestCompareVersionsRejectsUnparsable(t *testing.T) {
	invalid := []string{"", "dev", "junk", "1", "1.2", "1.2.3.4", "1.2.x", "1..3", "-1.2.3", "1.+2.3",
		"1.2.3-", "1.2.3+build", "vv1.2.3", " 1.2.3", "99999999999999999999.0.0"}
	for _, version := range invalid {
		t.Run(version, func(t *testing.T) {
			if _, err := compareVersions(version, "1.0.0"); err == nil {
				t.Fatalf("expected an error for %q as a", version)
			}
			if _, err := compareVersions("1.0.0", version); err == nil {
				t.Fatalf("expected an error for %q as b", version)
			}
		})
	}
}
