package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"howett.net/plist"
)

func main() {
	// Specify the directory path
	dirPath := "/Applications"

	// Open the main directory
	dir, err := os.Open(dirPath)
	if err != nil {
		fmt.Println("Error opening directory:", err)
		return
	}
	defer func(dir *os.File) {
		err := dir.Close()
		if err != nil {
			fmt.Println("Error closing directory:", err)
		}
	}(dir)

	// Read the directory entries
	fileInfos, err := dir.Readdir(0)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return
	}

	// Slices to store executables based on architectures
	var arm64Executables []string
	var x8664executables []string
	var universalExecutables []string

	// Iterate through the entries and parse Info.plist
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			// Check if Info.plist exists in the directory
			infoPlistPath := filepath.Join(dirPath, fileInfo.Name(), "Contents", "Info.plist")
			if _, err := os.Stat(infoPlistPath); err == nil {
				// Parse Info.plist
				cfBundleExecutable, err := parseInfoPlist(infoPlistPath)
				if err != nil {
					fmt.Printf("Error parsing %s: %v\n", infoPlistPath, err)
				} else {
					// fmt.Printf("CFBundleExecutable for %s: %s\n", infoPlistPath, cfBundleExecutable)

					// Check for the existence of a file with CFBundleExecutable under the macOS directory
					macOSFilePath := filepath.Join(dirPath, fileInfo.Name(), "Contents", "MacOS", cfBundleExecutable)
					if _, err := os.Stat(macOSFilePath); err == nil {
						// fmt.Printf("Executable file found at: %s\n", macOSFilePath)

						// Get architecture using the file command
						arch, err := getExecutableArchitecture(macOSFilePath)
						if err != nil {
							fmt.Println("Error determining architecture:", err)
						} else {
							// fmt.Printf("Architecture of %s: %s\n", macOSFilePath, arch)

							// Categorize executables based on architecture
							switch {
							case strings.Contains(arch, "executable arm64"):
								arm64Executables = append(arm64Executables, macOSFilePath)
							case strings.Contains(arch, "64-bit executable x86_64"):
								x8664executables = append(x8664executables, macOSFilePath)
							case strings.Contains(arch, "universal"):
								universalExecutables = append(universalExecutables, macOSFilePath)
							}
						}
					} else {
						fmt.Printf("Executable file not found at: %s\n", macOSFilePath)
					}
				}
			}
		}
	}

	// Sort the executable names in each slice
	sort.Strings(arm64Executables)
	sort.Strings(x8664executables)
	sort.Strings(universalExecutables)

	// Print the categorized and sorted executables
	fmt.Println("Intel Binaries")
	printExecutables(x8664executables)

	fmt.Println("\nApple Binaries")
	printExecutables(arm64Executables)

	fmt.Println("\nUniversal Binaries")
	printExecutables(universalExecutables)
}

func parseInfoPlist(filePath string) (string, error) {
	// Read the content of the Info.plist file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Try to unmarshal as plist
	var plistData map[string]interface{}
	_, err = plist.Unmarshal(content, &plistData)
	if err != nil {
		return "", fmt.Errorf("error parsing %s: %v", filePath, err)
	}

	// Access the value of CFBundleExecutable from the map
	if cfBundleExecutable, ok := plistData["CFBundleExecutable"].(string); ok {
		return cfBundleExecutable, nil
	}

	return "", fmt.Errorf("CFBundleExecutable not found in %s", filePath)
}

func getExecutableArchitecture(filePath string) (string, error) {
	// Run the file command to get the architecture
	cmd := exec.Command("file", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Extract architecture information from the output
	arch := extractArchitecture(string(output))
	return arch, nil
}

func extractArchitecture(output string) string {
	// Example output: "/path/to/executable: Mach-O 64-bit executable x86_64"
	parts := strings.Fields(output)
	for _, part := range parts {
		if part == "universal" {
			return "universal"
		}
	}

	if len(parts) >= 5 {
		return strings.Join(parts[3:], " ")
	}
	return "unknown"
}

func printExecutables(executables []string) {
	for _, exe := range executables {
		fmt.Println(exe)
	}
}
