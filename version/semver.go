package version

import (
	"fmt"
	"strconv"
	"strings"
)

type Semver struct {
	Major int
	Minor int
	Patch int
}

func NewSemVer(verToParse string, prefixes ...string) (*Semver, error) {
	for _, p := range prefixes {
		verToParse = strings.TrimPrefix(verToParse, p)
	}

	parts := strings.Split(verToParse, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version: %s", verToParse)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", verToParse)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", verToParse)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", verToParse)
	}

	return &Semver{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

func (sv *Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", sv.Major, sv.Minor, sv.Patch)
}

func (sv *Semver) Equal(version *Semver) bool {
	return sv.String() == version.String()
}

func (sv *Semver) LessThan(other *Semver) bool {
	if sv.Major != other.Major {
		return sv.Major < other.Major
	}
	if sv.Minor != other.Minor {
		return sv.Minor < other.Minor
	}
	return sv.Patch < other.Patch
}

func (sv *Semver) GreaterThan(other *Semver) bool {
	if sv.Major != other.Major {
		return sv.Major > other.Major
	}
	if sv.Minor != other.Minor {
		return sv.Minor > other.Minor
	}
	return sv.Patch > other.Patch
}
