// Copyright (C) 2014 The Syncthing Authors.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for
// more details.
//
// You should have received a copy of the GNU General Public License along
// with this program. If not, see <http://www.gnu.org/licenses/>.

package versioner

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/syncthing/syncthing/internal/config"
	"github.com/syncthing/syncthing/internal/osutil"
)

func init() {
	// Register the constructor for this type of versioner with the name "simple"
	Factories["simple"] = NewSimple
}

// The type holds our configuration
type Simple struct {
	keep       int
	folderPath string
}

// The constructor function takes a map of parameters and creates the type.
func NewSimple(folderID, folderPath string, params map[string]string) Versioner {
	keep, err := strconv.Atoi(params["keep"])
	if err != nil {
		keep = 5 // A reasonable default
	}

	s := Simple{
		keep:       keep,
		folderPath: folderPath,
	}

	if debug {
		l.Debugf("instantiated %#v", s)
	}
	return s
}

// Move away the named file to a version archive. If this function returns
// nil, the named file does not exist any more (has been archived).
func (v Simple) Archive(filePath string) error {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if debug {
				l.Debugln("not archiving nonexistent file", filePath)
			}
			return nil
		}
		return err
	}

	versionsDir := filepath.Join(v.folderPath, config.MarkerDir, config.VersionsDir)
	_, err = os.Stat(versionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			if debug {
				l.Debugln("creating versions dir", versionsDir)
			}
			os.MkdirAll(versionsDir, 0755)
			osutil.HideFile(versionsDir)
		} else {
			return err
		}
	}

	if debug {
		l.Debugln("archiving", filePath)
	}

	file := filepath.Base(filePath)
	inFolderPath, err := filepath.Rel(v.folderPath, filepath.Dir(filePath))
	if err != nil {
		return err
	}

	dir := filepath.Join(versionsDir, inFolderPath)
	err = os.MkdirAll(dir, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	ver := taggedFilename(file, fileInfo.ModTime().Format(TimeFormat))
	dst := filepath.Join(dir, ver)
	if debug {
		l.Debugln("moving to", dst)
	}
	err = osutil.Rename(filePath, dst)
	if err != nil {
		return err
	}

	// Glob according to the new file~timestamp.ext pattern.
	newVersions, err := filepath.Glob(filepath.Join(dir, taggedFilename(file, TimeGlob)))
	if err != nil {
		l.Warnln("globbing:", err)
		return nil
	}

	// Also according to the old file.ext~timestamp pattern.
	oldVersions, err := filepath.Glob(filepath.Join(dir, file+"~"+TimeGlob))
	if err != nil {
		l.Warnln("globbing:", err)
		return nil
	}

	// Use all the found filenames. "~" sorts after "." so all old pattern
	// files will be deleted before any new, which is as it should be.
	versions := uniqueSortedStrings(append(oldVersions, newVersions...))

	if len(versions) > v.keep {
		for _, toRemove := range versions[:len(versions)-v.keep] {
			if debug {
				l.Debugln("cleaning out", toRemove)
			}
			err = os.Remove(toRemove)
			if err != nil {
				l.Warnln("removing old version:", err)
			}
		}
	}

	return nil
}
