package main

import (
	"debug/macho"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestClassifyCPUs(t *testing.T) {
	tests := []struct {
		name string
		cpus []macho.Cpu
		want archCategory
	}{
		{"arm64 only", []macho.Cpu{macho.CpuArm64}, archApple},
		{"x86_64 only", []macho.Cpu{macho.CpuAmd64}, archIntel},
		{"arm64 and x86_64", []macho.Cpu{macho.CpuAmd64, macho.CpuArm64}, archUniversal},
		{"arm64 and arm64e", []macho.Cpu{macho.CpuArm64, macho.CpuArm64}, archApple},
		{"x86_64 and i386", []macho.Cpu{macho.CpuAmd64, macho.Cpu386}, archIntel},
		{"i386 only", []macho.Cpu{macho.Cpu386}, archOther},
		{"no cpus", nil, archOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyCPUs(tt.cpus); got != tt.want {
				t.Errorf("classifyCPUs(%v) = %v, want %v", tt.cpus, got, tt.want)
			}
		})
	}
}

func TestExecutableArchScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := executableArch(path)
	if err != nil {
		t.Fatalf("executableArch(script) error = %v", err)
	}
	if got != archOther {
		t.Errorf("executableArch(script) = %v, want %v", got, archOther)
	}
}

func TestExecutableArchMissing(t *testing.T) {
	if _, err := executableArch(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("executableArch(missing) error = nil, want error")
	}
}

func TestExecutableArchThin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Mach-O test binary requires macOS")
	}

	// The test binary itself is a thin Mach-O for the host architecture
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	want := archApple
	if runtime.GOARCH == "amd64" {
		want = archIntel
	}

	got, err := executableArch(exe)
	if err != nil {
		t.Fatalf("executableArch(%s) error = %v", exe, err)
	}
	if got != want {
		t.Errorf("executableArch(%s) = %v, want %v", exe, got, want)
	}
}

func TestExecutableArchFat(t *testing.T) {
	// macOS system binaries ship as arm64e + x86_64 fat files
	const path = "/usr/bin/true"
	if _, err := macho.OpenFat(path); err != nil {
		t.Skipf("%s is not a fat Mach-O on this system: %v", path, err)
	}

	got, err := executableArch(path)
	if err != nil {
		t.Fatalf("executableArch(%s) error = %v", path, err)
	}
	if got != archUniversal {
		t.Errorf("executableArch(%s) = %v, want %v", path, got, archUniversal)
	}
}

func TestParseInfoPlistInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := os.WriteFile(path, []byte("<plist><dict><key>"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := parseInfoPlist(path)
	if err == nil {
		t.Fatal("parseInfoPlist(invalid) error = nil, want error")
	}
	if n := strings.Count(err.Error(), path); n != 1 {
		t.Errorf("parseInfoPlist(invalid) error %q mentions path %d times, want 1", err, n)
	}
}

// makeBundle creates a minimal app bundle with an Info.plist at path.
func makeBundle(t *testing.T, path string) {
	t.Helper()
	contents := filepath.Join(path, "Contents")
	if err := os.MkdirAll(contents, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte("<plist><dict/></plist>"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFindBundles(t *testing.T) {
	root := t.TempDir()
	makeBundle(t, filepath.Join(root, "Top.app"))
	makeBundle(t, filepath.Join(root, "Utilities", "Nested.app"))
	makeBundle(t, filepath.Join(root, "Top.app", "Contents", "Helpers", "Helper.app"))
	if err := os.MkdirAll(filepath.Join(root, "Empty"), 0o750); err != nil {
		t.Fatal(err)
	}

	elsewhere := t.TempDir()
	makeBundle(t, filepath.Join(elsewhere, "Linked.app"))
	if err := os.Symlink(filepath.Join(elsewhere, "Linked.app"), filepath.Join(root, "Linked.app")); err != nil {
		t.Fatal(err)
	}
	// A symlink back to root must not cause an endless walk
	if err := os.Symlink(root, filepath.Join(root, "Loop")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		recursive bool
		want      []string
	}{
		{"top level", false, []string{"Linked.app", "Top.app"}},
		{"recursive", true, []string{"Linked.app", "Top.app", filepath.Join("Utilities", "Nested.app")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findBundles(root, tt.recursive)
			if err != nil {
				t.Fatalf("findBundles error = %v", err)
			}
			want := make([]string, len(tt.want))
			for i, rel := range tt.want {
				want[i] = filepath.Join(root, rel)
			}
			if !slices.Equal(got, want) {
				t.Errorf("findBundles = %v, want %v", got, want)
			}
		})
	}
}

func TestRunMissingRoot(t *testing.T) {
	if err := run([]string{filepath.Join(t.TempDir(), "missing")}, false); err == nil {
		t.Error("run(missing root) error = nil, want error")
	}
}
