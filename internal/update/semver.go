package update

import (
	"fmt"
	"strconv"
	"strings"
)

type semver struct {
	parts      [3]int
	prerelease string
}

// parseSemver accepts MAJOR.MINOR.PATCH with an optional leading "v" and an optional
// "-prerelease" suffix. Anything else, build metadata included, is an error.
func parseSemver(raw string) (semver, error) {
	core, prerelease, hasPrerelease := strings.Cut(strings.TrimPrefix(raw, "v"), "-")
	fields := strings.Split(core, ".")
	if len(fields) != 3 || (hasPrerelease && prerelease == "") {
		return semver{}, fmt.Errorf("invalid version %q: want MAJOR.MINOR.PATCH", raw)
	}
	var parsed semver
	for index, field := range fields {
		if field == "" || strings.Trim(field, "0123456789") != "" {
			return semver{}, fmt.Errorf("invalid version %q: %q is not a number", raw, field)
		}
		number, err := strconv.Atoi(field)
		if err != nil {
			return semver{}, fmt.Errorf("invalid version %q: %w", raw, err)
		}
		parsed.parts[index] = number
	}
	parsed.prerelease = prerelease
	return parsed, nil
}

// compareVersions returns -1, 0 or 1 as a is lower than, equal to or higher than b. A
// prerelease sorts below its release, and two prereleases compare lexically.
func compareVersions(a string, b string) (int, error) {
	left, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	right, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for index := range left.parts {
		if left.parts[index] != right.parts[index] {
			return sign(left.parts[index] - right.parts[index]), nil
		}
	}
	switch {
	case left.prerelease == right.prerelease:
		return 0, nil
	case left.prerelease == "":
		return 1, nil
	case right.prerelease == "":
		return -1, nil
	}
	return strings.Compare(left.prerelease, right.prerelease), nil
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	return 1
}
