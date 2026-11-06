package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsAutostartLaunch(t *testing.T) {
	directoryArg := autostartDirectoryArg + filepath.Join(t.TempDir(), "reader books")
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"manual", nil, false},
		{"manual with book", []string{"a book.txt"}, false},
		{"similar flag", []string{"--autostart=false"}, false},
		{"relative directory", []string{autostartDirectoryArg + "books"}, false},
		{"empty directory", []string{autostartDirectoryArg}, false},
		{"explicit autostart", []string{autostartArg}, true},
		{"legacy Windows registration", []string{directoryArg}, runtime.GOOS == "windows"},
		{"new registration", []string{autostartArg, directoryArg}, true},
		{"second instance", []string{"wdreader.exe", directoryArg, autostartArg}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAutostartLaunch(tc.args); got != tc.want {
				t.Fatalf("isAutostartLaunch(%q) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestRestoreAutostartDirectory(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows restores the autostart working directory")
	}
	root := t.TempDir()
	target := filepath.Join(root, "reader books")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	directoryArg := autostartDirectoryArg + target
	for _, tc := range []struct {
		name    string
		args    []string
		wantDir string
		wantErr bool
	}{
		{"manual", nil, root, false},
		{"relative directory", []string{autostartDirectoryArg + "reader books"}, root, false},
		{"legacy registration", []string{directoryArg}, target, false},
		{"new registration", []string{autostartArg, directoryArg}, target, false},
		{"reordered arguments", []string{directoryArg, autostartArg}, target, false},
		{"missing directory", []string{autostartArg, autostartDirectoryArg + filepath.Join(root, "missing")}, root, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(root)
			if err := restoreAutostartDirectory(tc.args); (err != nil) != tc.wantErr {
				t.Fatalf("restoreAutostartDirectory(%q) error = %v", tc.args, err)
			}
			got, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.wantDir {
				t.Fatalf("working directory = %q, want %q", got, tc.wantDir)
			}
		})
	}
}
