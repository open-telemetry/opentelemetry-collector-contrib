package servermondataformatter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Read10min(directory string) []string {
	// Slice to store found files
	var foundFiles []string

	// Function to walk through directory and find files
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the file is a regular file and contains '10min-show' in its name
		if info.Mode().IsRegular() && strings.Contains(filepath.Base(path), "10min-show") {
			// If found, add the file path to the slice
			foundFiles = append(foundFiles, path)
		}
		return nil
	}

	// Walk through the directory
	err := filepath.Walk(directory, walkFunc)
	if err != nil {
		fmt.Println("Error:", err)
	}

	return foundFiles
}
