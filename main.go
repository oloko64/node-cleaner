package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
)

func main() {
	invertedSelection := flag.Bool("invert", false, "Invert selection (by default, all found directories are selected for deletion)")
	flag.Parse()

	color.Magenta("Version: 0.1.110\n")

	color.Cyan("Searching for node_modules directories...")
	color.Cyan("This may take a while depending on the size of the recursion.\n\n")
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return
	}

	files, err := findInParallel(cwd)
	if err != nil {
		color.Red("Error finding files: %v", err)
	}
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

	defer func() {
		err = runYarnCacheClean()
		if err != nil {
			color.Red("Error running 'yarn cache clean --all': %v", err)
		}
	}()

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
		semaphore <- struct{}{}

		spaceSaved := file.SizeMB
		totalSpaceSaved += spaceSaved

		wg.Go(func() {
			defer func() { <-semaphore }()

			err = os.RemoveAll(file.FullPath)
			if err != nil {
				color.Red("Error removing %s: %v\n", file.FullPath, err)
			} else {
				color.Green("Successfully removed %s, freed %dMB\n", file.FullPath, spaceSaved)
			}
		})
	}
	wg.Wait()

	color.Green("\nTotal space freed: %dMB\n", totalSpaceSaved)
}

func findInParallel(cwd string) (FoundNodeModules, error) {
	var files FoundNodeModules
	var mu sync.Mutex
	var wg sync.WaitGroup

	jobs := make(chan string, 100)

	for range 10 {
		wg.Go(func() {
			for path := range jobs {
				foundFile, err := processPackageJson(path)
				if err != nil {
					color.Yellow("%v", err)
					continue
				}
				mu.Lock()
				files = append(files, *foundFile)
				mu.Unlock()
			}
		})
	}

	err := filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "node_modules" {
			jobs <- path
			return filepath.SkipDir
		}
		return nil
	})

	close(jobs)
	wg.Wait()

	return files, err

}

func processPackageJson(path string) (*FoundNodeModule, error) {
	packageJsonPath := filepath.Join(filepath.Dir(path), "package.json")
	content, err := os.Open(packageJsonPath)
	if err != nil {
		isNotExist := os.IsNotExist(err)
		if isNotExist {
			return nil, fmt.Errorf("package.json not found in path: %s", packageJsonPath)
		}
		return nil, err
	}
	defer content.Close()

	var pkg PackageJson
	if err := json.NewDecoder(content).Decode(&pkg); err != nil {
		return nil, err
	}

	return &FoundNodeModule{
		FullPath:        path,
		Name:            "node_modules",
		Dependencies:    len(pkg.Dependencies),
		DevDependencies: len(pkg.DevDependencies),
	}, nil
}

func runYarnCacheClean() error {
	// Ask user for confirmation
	var response string
	fmt.Print("\nDo you want to run 'yarn cache clean --all' to free up additional space? (y/N): ")
	_, err := fmt.Scanln(&response)
	if err != nil {
		if err.Error() == "unexpected newline" {
			color.Yellow("Skipping 'yarn cache clean --all'.")
			return nil
		}
		return err
	}
	if response == "y" || response == "Y" || response == "yes" || response == "YES" {
		color.Cyan("Running 'yarn cache clean --all'...")
		cmd := exec.Command("yarn", "cache", "clean", "--all")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error running 'yarn cache clean --all': %v", err)
		}
		color.Green("'yarn cache clean --all' completed successfully.")
	} else {
		color.Yellow("Skipping 'yarn cache clean --all'.")
	}
	return nil
}
