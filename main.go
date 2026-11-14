package main

import (
	"debug/macho"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"howett.net/plist"
)

func main() {
	recursive := flag.Bool("r", false, "search subdirectories for app bundles")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-r] [dir ...]\n\nDirectories default to /Applications.\n\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"/Applications"}
	}

	if err := run(roots, *recursive); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(roots []string, recursive bool) error {
	// Collect app bundles from every directory to scan
	var bundles []string
	for _, root := range roots {
		found, err := findBundles(root, recursive)
		if err != nil {
			return err
		}
		bundles = append(bundles, found...)
	}

	// Slices to store executables based on architectures
	var arm64Executables []string
	var x8664executables []string
	var universalExecutables []string
	var otherExecutables []string

	// Parse each bundle's Info.plist and classify its executable
	for _, bundle := range bundles {
		cfBundleExecutable, err := parseInfoPlist(filepath.Join(bundle, "Contents", "Info.plist"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			continue
		}

		// Check for the existence of a file with CFBundleExecutable under the macOS directory
		macOSFilePath := filepath.Join(bundle, "Contents", "MacOS", cfBundleExecutable)
		if _, err := os.Stat(macOSFilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Executable file not found at: %s\n", macOSFilePath)
			continue
		}

		// Read the architectures from the Mach-O headers
		arch, err := executableArch(macOSFilePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error determining architecture:", err)
			continue
		}

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

// findBundles returns the app bundles directly inside dir, or anywhere below
// it when recursive is set. Bundles are not searched for nested bundles.
func findBundles(dir string, recursive bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var bundles []string
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if entry.Type()&fs.ModeSymlink != 0 {
			// Follow symlinks to bundles only, so recursion can't loop
			if info, err := os.Stat(path); err == nil && info.IsDir() && isBundle(path) {
				bundles = append(bundles, path)
			}
			continue
		}
		if !entry.IsDir() {
			continue
		}

		switch {
		case isBundle(path):
			bundles = append(bundles, path)
		case recursive:
			nested, err := findBundles(path, true)
			if err != nil {
				// Unreadable subdirectories are skipped, not fatal
				fmt.Fprintln(os.Stderr, "Error:", err)
				continue
			}
			bundles = append(bundles, nested...)
		}
	}
	return bundles, nil
}

func isBundle(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Contents", "Info.plist"))
	return err == nil
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
