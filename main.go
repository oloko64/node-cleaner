package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
)

func main() {
	invertedSelection := flag.Bool("invert", false, "Invert selection (by default, all found directories are selected for deletion)")
	flag.Parse()

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
	options := []huh.Option[string]{}

	for i := range files {
		dirSize, err := files[i].GetDirSizeMB()
		if err != nil {
			color.Red("Error getting size for %s: %v\n", files[i].FullPath, err)
		}

		label := fmt.Sprintf("%s (%dMB, %d dependencies)", files[i].FullPath, dirSize, files[i].Dependencies+files[i].DevDependencies)
		options = append(options, huh.NewOption(label, files[i].FullPath).Selected(!*invertedSelection))

		totalSize += dirSize
	}

	if len(files) == 0 {
		color.Green("No node_modules directories found.")
		return
	}

	color.Cyan("\nFound %d node_modules directories consuming a total of %dMB\n", len(files), totalSize)

	var selected []string
	multiSelect := huh.NewMultiSelect[string]().
		Options(options...).
		Title("Select node_modules directories to remove:").
		Value(&selected)
	if err := multiSelect.Run(); err != nil {
		fmt.Println("Error during selection:", err)
		return
	}

	if len(selected) == 0 {
		color.Yellow("No directories selected for removal. Exiting.")
		return
	}

	var filesToRemove FoundNodeModules
	for _, sel := range selected {
		for _, file := range files {
			if file.FullPath == sel {
				filesToRemove = append(filesToRemove, file)
				break
			}
		}
	}

	var totalSpaceSaved int64
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent deletions
	for _, file := range filesToRemove {
		wg.Add(1)
		spaceSaved := file.SizeMB
		totalSpaceSaved += spaceSaved

		go func() {
			defer wg.Done()
			defer func() { <-semaphore }()

			err = os.RemoveAll(file.FullPath)
			if err != nil {
				color.Red("Error removing %s: %v\n", file.FullPath, err)
			} else {
				color.Green("Successfully removed %s, freed %dMB\n", file.FullPath, spaceSaved)
			}
		}()
	}
	wg.Wait()

	color.Green("\nTotal space freed: %dMB\n", totalSpaceSaved)
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
