// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dump

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/timeutil"
)

var SupportedOutputTypes = []string{"zip", "tar", "tar.gz"}

// PrepareFileNameAndType prepares the output file name and type, if the type is not supported, it returns an empty "outType"
func PrepareFileNameAndType(argFile, argType string) (outFileName, outType string) {
	if argFile == "" && argType == "" {
		outType = SupportedOutputTypes[0]
		outFileName = fmt.Sprintf("gitea-dump-%d.%s", timeutil.TimeStampNow(), outType)
	} else if argFile == "" {
		outType = argType
		outFileName = fmt.Sprintf("gitea-dump-%d.%s", timeutil.TimeStampNow(), outType)
	} else if argType == "" {
		if filepath.Ext(outFileName) == "" {
			outType = SupportedOutputTypes[0]
			outFileName = argFile
		} else {
			for _, t := range SupportedOutputTypes {
				if strings.HasSuffix(argFile, "."+t) {
					outFileName = argFile
					outType = t
				}
			}
		}
	} else {
		outFileName, outType = argFile, argType
	}
	if !slices.Contains(SupportedOutputTypes, outType) {
		return "", ""
	}
	return outFileName, outType
}

func IsSubdir(upper, lower string) (bool, error) {
	if relPath, err := filepath.Rel(upper, lower); err != nil {
		return false, err
	} else if relPath == "." || !strings.HasPrefix(relPath, ".") {
		return true, nil
	}
	return false, nil
}

type Dumper struct {
	Verbose bool

	zipWriter             *zip.Writer
	tarWriter             *tar.Writer
	compressedTarWriter   *gzip.Writer
	globalExcludeAbsPaths []string
}

func NewDumper(ctx context.Context, format string, output io.Writer) (*Dumper, error) {
	_ = ctx

	d := &Dumper{
		Verbose: false,
	}

	switch format {
	case "zip":
		d.zipWriter = zip.NewWriter(output)
	case "tar":
		d.tarWriter = tar.NewWriter(output)
	case "tar.gz":
		d.compressedTarWriter = gzip.NewWriter(output)
		d.tarWriter = tar.NewWriter(d.compressedTarWriter)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	return d, nil
}

// AddFileByPath adds a file by its filesystem path
func (dumper *Dumper) AddFileByPath(filePath, absPath string) error {
	if dumper.Verbose {
		log.Info("Adding local file %s", filePath)
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return dumper.writeEntry(filePath, fileInfo, nil)
	}
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return dumper.writeEntry(filePath, fileInfo, file)
}

type readerFile struct {
	r    io.Reader
	info os.FileInfo
}

var _ fs.File = (*readerFile)(nil)

func (f *readerFile) Stat() (fs.FileInfo, error)     { return f.info, nil }
func (f *readerFile) Read(bytes []byte) (int, error) { return f.r.Read(bytes) }
func (f *readerFile) Close() error                   { return nil }

// AddFileByReader adds a file's contents from a Reader
func (dumper *Dumper) AddFileByReader(r io.Reader, info os.FileInfo, customName string) error {
	if dumper.Verbose {
		log.Info("Adding storage file %s", customName)
	}

	return dumper.writeEntry(customName, info, &readerFile{r, info})
}

func (dumper *Dumper) Close() error {
	var err error
	if dumper.tarWriter != nil {
		err = errors.Join(err, dumper.tarWriter.Close())
	}
	if dumper.compressedTarWriter != nil {
		err = errors.Join(err, dumper.compressedTarWriter.Close())
	}
	if dumper.zipWriter != nil {
		err = errors.Join(err, dumper.zipWriter.Close())
	}
	return err
}

func (dumper *Dumper) writeEntry(name string, info os.FileInfo, src io.Reader) error {
	if dumper.zipWriter != nil {
		return dumper.writeZipEntry(name, info, src)
	}
	if dumper.tarWriter != nil {
		return dumper.writeTarEntry(name, info, src)
	}
	return fmt.Errorf("unsupported dumper writer")
}

func (dumper *Dumper) writeZipEntry(name string, info os.FileInfo, src io.Reader) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	writer, err := dumper.zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	if src == nil {
		return nil
	}
	_, err = io.Copy(writer, src)
	return err
}

func (dumper *Dumper) writeTarEntry(name string, info os.FileInfo, src io.Reader) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = name
	if src == nil {
		header.Size = 0
	}
	if err := dumper.tarWriter.WriteHeader(header); err != nil {
		return err
	}
	if src == nil {
		return nil
	}
	_, err = io.Copy(dumper.tarWriter, src)
	return err
}

func (dumper *Dumper) normalizeFilePath(absPath string) string {
	absPath = filepath.Clean(absPath)
	if setting.IsWindows {
		absPath = strings.ToLower(absPath)
	}
	return absPath
}

func (dumper *Dumper) GlobalExcludeAbsPath(absPaths ...string) {
	for _, absPath := range absPaths {
		dumper.globalExcludeAbsPaths = append(dumper.globalExcludeAbsPaths, dumper.normalizeFilePath(absPath))
	}
}

func (dumper *Dumper) shouldExclude(absPath string, excludes []string) bool {
	norm := dumper.normalizeFilePath(absPath)
	return slices.Contains(dumper.globalExcludeAbsPaths, norm) || slices.Contains(excludes, norm)
}

func (dumper *Dumper) AddRecursiveExclude(insidePath, absPath string, excludes []string) error {
	excludes = slices.Clone(excludes)
	for i := range excludes {
		excludes[i] = dumper.normalizeFilePath(excludes[i])
	}
	return dumper.addFileOrDir(insidePath, absPath, excludes)
}

func (dumper *Dumper) addFileOrDir(insidePath, absPath string, excludes []string) error {
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return err
	}
	dir, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	for _, file := range files {
		currentAbsPath := filepath.Join(absPath, file.Name())
		if dumper.shouldExclude(currentAbsPath, excludes) {
			continue
		}

		currentInsidePath := path.Join(insidePath, file.Name())
		if file.IsDir() {
			if err := dumper.AddFileByPath(currentInsidePath, currentAbsPath); err != nil {
				return err
			}
			if err = dumper.addFileOrDir(currentInsidePath, currentAbsPath, excludes); err != nil {
				return err
			}
		} else {
			// only copy regular files and symlink regular files, skip non-regular files like socket/pipe/...
			shouldAdd := file.Mode().IsRegular()
			if !shouldAdd && file.Mode()&os.ModeSymlink == os.ModeSymlink {
				target, err := filepath.EvalSymlinks(currentAbsPath)
				if err != nil {
					return err
				}
				targetStat, err := os.Stat(target)
				if err != nil {
					return err
				}
				shouldAdd = targetStat.Mode().IsRegular()
			}
			if shouldAdd {
				if err = dumper.AddFileByPath(currentInsidePath, currentAbsPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
