// Copyright 2026 The GitJet Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package semver

import (
	"fmt"
	"strconv"
	"strings"

	msv "github.com/Masterminds/semver/v3"
)

// Version is a compatibility wrapper used to replace hashicorp/go-version.
type Version struct {
	v        *msv.Version
	original string
}

func normalizeInput(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "v") || strings.HasPrefix(s, "V") {
		s = s[1:]
	}
	mainAndMeta := strings.SplitN(s, "+", 2)
	mainAndPre := strings.SplitN(mainAndMeta[0], "-", 2)
	parts := strings.Split(mainAndPre[0], ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	main := strings.Join(parts, ".")
	if len(mainAndPre) == 2 {
		main += "-" + mainAndPre[1]
	}
	if len(mainAndMeta) == 2 {
		main += "+" + mainAndMeta[1]
	}
	return main
}

func parseVersion(s string) (*Version, error) {
	n := normalizeInput(s)
	v, err := msv.NewVersion(n)
	if err != nil {
		return nil, err
	}
	return &Version{v: v, original: s}, nil
}

func NewVersion(s string) (*Version, error) {
	return parseVersion(s)
}

func NewSemver(s string) (*Version, error) {
	return parseVersion(s)
}

func Must(v *Version, err error) *Version {
	if err != nil {
		panic(err)
	}
	return v
}

func (v *Version) Compare(other *Version) int {
	return v.v.Compare(other.v)
}

func (v *Version) LessThan(other *Version) bool {
	return v.v.LessThan(other.v)
}

func (v *Version) Equal(other *Version) bool {
	return v.v.Equal(other.v)
}

func (v *Version) String() string {
	return v.v.String()
}

func (v *Version) Original() string {
	if strings.TrimSpace(v.original) == "" {
		return v.v.Original()
	}
	return v.original
}

func (v *Version) Segments64() []int64 {
	parts := strings.Split(v.v.String(), "-")
	main := strings.Split(parts[0], ".")
	out := make([]int64, 0, len(main))
	for _, p := range main {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("invalid numeric version segment %q: %v", p, err))
		}
		out = append(out, n)
	}
	return out
}

func (v *Version) Prerelease() string {
	return v.v.Prerelease()
}

func (v *Version) Core() *Version {
	seg := v.Segments64()
	core, err := NewVersion(fmt.Sprintf("%d.%d.%d", seg[0], seg[1], seg[2]))
	if err != nil {
		panic(err)
	}
	core.original = core.String()
	return core
}
