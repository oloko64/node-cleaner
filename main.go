package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fatih/color"
)

func main() {
	color.Magenta("Version: 0.1.1\n")

	color.Cyan("Searching for node_modules directories...")
	color.Cyan("This may take a while depending on the size of the recursion.\n\n")
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return
	}

	files := findFiles(cwd)
	files = files.OrganizeByDependenciesNum()

	var totalSize int64
	for i := range files {
		dirSize, err := files[i].GetDirSizeMB()
		if err != nil {
			color.Red("Error getting size for %s: %v\n", files[i].FullPath, err)
		}
		totalSize += dirSize
		color.Magenta("Path: %s\n", files[i].FullPath)
		color.Magenta("├── Size: %dMB\n", dirSize)
		color.Magenta("└── Total Dependencies: %d\n", files[i].Dependencies+files[i].DevDependencies)
	}

	color.Cyan("\nFound %d node_modules directories consuming a total of %dMB\n", len(files), totalSize)

	if len(files) == 0 {
		color.Green("No node_modules directories found.")
		return
	}

	if askToRemove() {
		var totalSpaceSaved int64
		for _, file := range files {
			spaceSaved := file.SizeMB
			totalSpaceSaved += spaceSaved

			err = os.RemoveAll(file.FullPath)
			if err != nil {
				color.Red("Error removing %s: %v\n", file.FullPath, err)
			} else {
				color.Green("Successfully removed %s, freed %dMB\n", file.FullPath, spaceSaved)
			}
		}
		color.Green("\nTotal space freed: %dMB\n", totalSpaceSaved)
	} else {
		color.Yellow("No directories were removed.")
	}
}

func askToRemove() bool {
	var response string
	fmt.Print("\nDo you want to remove these node_modules directories? (y/N): ")
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}
	return response == "y" || response == "Y"
}

func findFiles(cwd string) FoundNodeModules {
	// The code needs to recursive search for files in the given directory (cwd)
	// and return a slice of FoundFile structs representing each found file.
	// The pattern is to find all folders called "node_modules" only if in the same directory there is a "package.json" file.
	var files FoundNodeModules

	err := filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && d.Name() == "node_modules" {
			packageJsonPath := filepath.Join(filepath.Dir(path), "package.json")
			if _, err := os.Stat(packageJsonPath); err == nil {
				content, err := os.Open(packageJsonPath)
				if err != nil {
					return err
				}
				defer content.Close()

				var depCount int
				var devDepCount int
				var pkg PackageJson
				err = json.NewDecoder(content).Decode(&pkg)
				if err != nil {
					return err
				}

				depCount += len(pkg.Dependencies)
				devDepCount += len(pkg.DevDependencies)

				files = append(files, FoundNodeModule{
					FullPath:        path,
					Name:            d.Name(),
					Dependencies:    depCount,
					DevDependencies: devDepCount,
				})
			}

			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	return files
}
