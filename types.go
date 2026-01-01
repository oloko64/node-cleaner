package main

import (
	"io/fs"
	"path/filepath"
)

type (
	FoundNodeModule struct {
		FullPath        string
		Name            string
		SizeMB          int64
		Dependencies    int
		DevDependencies int
	}

	PackageJson struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	FoundNodeModules []FoundNodeModule
)

func (f *FoundNodeModule) String() string {
	return f.FullPath
}

func (f *FoundNodeModule) GetDirSizeMB() (int64, error) {
	var size int64
	err := filepath.WalkDir(f.FullPath, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})

	f.SizeMB = size / (1024 * 1024)

	return size / (1024 * 1024), err
}

func (f *FoundNodeModules) OrganizeByDependenciesNum() FoundNodeModules {
	// This function should organize the FoundNodeModules by size in descending order.
	// Implementing a simple bubble sort for demonstration purposes.
	n := len(*f)
	for i := range n {
		for j := 0; j < n-i-1; j++ {
			allDeps := (*f)[j].Dependencies + (*f)[j].DevDependencies
			nextDeps := (*f)[j+1].Dependencies + (*f)[j+1].DevDependencies
			if allDeps < nextDeps {
				(*f)[j], (*f)[j+1] = (*f)[j+1], (*f)[j]
			}
		}
	}
	return *f
}
