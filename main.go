package main

import (
	"debug/macho"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"howett.net/plist"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	// Specify the directory path
	dirPath := "/Applications"

	// Open the main directory
	dir, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("opening directory: %w", err)
	}
	defer func(dir *os.File) {
		err := dir.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error closing directory:", err)
		}
	}(dir)

	// Read the directory entries
	fileInfos, err := dir.Readdir(0)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	// Slices to store executables based on architectures
	var arm64Executables []string
	var x8664executables []string
	var universalExecutables []string
	var otherExecutables []string

	// Iterate through the entries and parse Info.plist
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			// Check if Info.plist exists in the directory
			infoPlistPath := filepath.Join(dirPath, fileInfo.Name(), "Contents", "Info.plist")
			if _, err := os.Stat(infoPlistPath); err == nil {
				// Parse Info.plist
				cfBundleExecutable, err := parseInfoPlist(infoPlistPath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
				} else {
					// fmt.Printf("CFBundleExecutable for %s: %s\n", infoPlistPath, cfBundleExecutable)

					// Check for the existence of a file with CFBundleExecutable under the macOS directory
					macOSFilePath := filepath.Join(dirPath, fileInfo.Name(), "Contents", "MacOS", cfBundleExecutable)
					if _, err := os.Stat(macOSFilePath); err == nil {
						// fmt.Printf("Executable file found at: %s\n", macOSFilePath)

						// Read the architectures from the Mach-O headers
						arch, err := executableArch(macOSFilePath)
						if err != nil {
							fmt.Fprintln(os.Stderr, "Error determining architecture:", err)
						} else {
							// Categorize executables based on architecture
							switch arch {
							case archApple:
								arm64Executables = append(arm64Executables, macOSFilePath)
							case archIntel:
								x8664executables = append(x8664executables, macOSFilePath)
							case archUniversal:
								universalExecutables = append(universalExecutables, macOSFilePath)
							default:
								otherExecutables = append(otherExecutables, macOSFilePath)
							}
						}
					} else {
						fmt.Fprintf(os.Stderr, "Executable file not found at: %s\n", macOSFilePath)
					}
				}
			}
		}
	}

	// Sort the executable names in each slice
	sort.Strings(arm64Executables)
	sort.Strings(x8664executables)
	sort.Strings(universalExecutables)
	sort.Strings(otherExecutables)

	// Print the categorized and sorted executables
	fmt.Println("Intel Binaries")
	printExecutables(x8664executables)

	fmt.Println("\nApple Binaries")
	printExecutables(arm64Executables)

	fmt.Println("\nUniversal Binaries")
	printExecutables(universalExecutables)

	fmt.Println("\nOther/Unknown")
	printExecutables(otherExecutables)

	return nil
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
		return "", fmt.Errorf("parsing %s: %w", filePath, err)
	}

	// Access the value of CFBundleExecutable from the map
	if cfBundleExecutable, ok := plistData["CFBundleExecutable"].(string); ok {
		return cfBundleExecutable, nil
	}

	return "", fmt.Errorf("CFBundleExecutable not found in %s", filePath)
}

// archCategory groups executables by the CPU architectures they contain.
type archCategory int

const (
	archOther archCategory = iota
	archApple
	archIntel
	archUniversal
)

func executableArch(filePath string) (archCategory, error) {
	// Try a fat (multi-architecture) binary first
	fat, err := macho.OpenFat(filePath)
	if err == nil {
		defer fat.Close() //nolint:errcheck // read-only file
		cpus := make([]macho.Cpu, 0, len(fat.Arches))
		for _, a := range fat.Arches {
			cpus = append(cpus, a.Cpu)
		}
		return classifyCPUs(cpus), nil
	}

	var formatErr *macho.FormatError
	if !errors.As(err, &formatErr) {
		return archOther, err
	}
	if err != macho.ErrNotFat {
		// Not a Mach-O file at all, e.g. a shell script launcher
		return archOther, nil
	}

	// Fall back to a thin (single-architecture) binary
	thin, err := macho.Open(filePath)
	if err != nil {
		return archOther, fmt.Errorf("reading %s: %w", filePath, err)
	}
	defer thin.Close() //nolint:errcheck // read-only file
	return classifyCPUs([]macho.Cpu{thin.Cpu}), nil
}

func classifyCPUs(cpus []macho.Cpu) archCategory {
	// arm64e shares CpuArm64 and i386 is ignored, so only arm64 + x86_64 is universal
	hasArm64 := slices.Contains(cpus, macho.CpuArm64)
	hasAmd64 := slices.Contains(cpus, macho.CpuAmd64)

	switch {
	case hasArm64 && hasAmd64:
		return archUniversal
	case hasArm64:
		return archApple
	case hasAmd64:
		return archIntel
	default:
		return archOther
	}
}

func printExecutables(executables []string) {
	for _, exe := range executables {
		fmt.Println(exe)
	}
}
