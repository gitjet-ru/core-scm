// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"errors"
	"io"

	"github.com/gitjet-ru/core-scm/modules/timeutil"
)

const (
	PropertyMetadata     = "rpm.metadata"
	PropertyGroup        = "rpm.group"
	PropertyArchitecture = "rpm.architecture"

	SettingKeyPrivate = "rpm.key.private"
	SettingKeyPublic  = "rpm.key.public"

	RepositoryPackage = "_rpm"
	RepositoryVersion = "_repository"
)

const (
	// Can't use the syscall constants because they are not available for windows build.
	sIFMT  = 0xf000
	sIFDIR = 0x4000
	sIXUSR = 0x40
	sIXGRP = 0x8
	sIXOTH = 0x1
)

// https://rpm-software-management.github.io/rpm/manual/spec.html
// https://refspecs.linuxbase.org/LSB_3.1.0/LSB-Core-generic/LSB-Core-generic/pkgformat.html

type Package struct {
	Name            string
	Version         string
	VersionMetadata *VersionMetadata
	FileMetadata    *FileMetadata
}

type VersionMetadata struct {
	License     string `json:"license,omitempty"`
	ProjectURL  string `json:"project_url,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
}

type FileMetadata struct {
	Architecture  string `json:"architecture,omitempty"`
	Epoch         string `json:"epoch,omitempty"`
	Version       string `json:"version,omitempty"`
	Release       string `json:"release,omitempty"`
	Vendor        string `json:"vendor,omitempty"`
	Group         string `json:"group,omitempty"`
	Packager      string `json:"packager,omitempty"`
	SourceRpm     string `json:"source_rpm,omitempty"`
	BuildHost     string `json:"build_host,omitempty"`
	BuildTime     uint64 `json:"build_time,omitempty"`
	FileTime      uint64 `json:"file_time,omitempty"`
	InstalledSize uint64 `json:"installed_size,omitempty"`
	ArchiveSize   uint64 `json:"archive_size,omitempty"`

	Provides  []*Entry `json:"provide,omitempty"`
	Requires  []*Entry `json:"require,omitempty"`
	Conflicts []*Entry `json:"conflict,omitempty"`
	Obsoletes []*Entry `json:"obsolete,omitempty"`

	Files []*File `json:"files,omitempty"`

	Changelogs []*Changelog `json:"changelogs,omitempty"`
}

type Entry struct {
	Name    string `json:"name" xml:"name,attr"`
	Flags   string `json:"flags,omitempty" xml:"flags,attr,omitempty"`
	Version string `json:"version,omitempty" xml:"ver,attr,omitempty"`
	Epoch   string `json:"epoch,omitempty" xml:"epoch,attr,omitempty"`
	Release string `json:"release,omitempty" xml:"rel,attr,omitempty"`
}

type File struct {
	Path         string `json:"path" xml:",chardata"`
	Type         string `json:"type,omitempty" xml:"type,attr,omitempty"`
	IsExecutable bool   `json:"is_executable" xml:"-"`
}

type Changelog struct {
	Author string             `json:"author,omitempty" xml:"author,attr"`
	Date   timeutil.TimeStamp `json:"date,omitempty" xml:"date,attr"`
	Text   string             `json:"text,omitempty" xml:",chardata"`
}

// ParsePackage parses the RPM package file
func ParsePackage(r io.Reader) (*Package, error) {
	_ = r
	return nil, errors.New("rpm metadata parsing is disabled in this build")
}
