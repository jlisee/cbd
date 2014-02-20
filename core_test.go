package cbuildd

import (
	"testing"
	"os"
	"strings"
	"path/filepath"
)

// Helper functions
func StrsEquals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

type TestCase struct {
	inputArgs []string
	b         Build // Resulting build object
}

func TestParseArgs(t *testing.T) {
	// Note args is left out of the Build struct because it supplied separately
	testData := []TestCase{
		TestCase{
			inputArgs: []string{"-c", "data/main.c", "-o", "main.o"},
			b: Build{
				Output:      "main.o",
				Oindex:      3,
				Iindex:      1,
				Cindex:      0,
				LinkCommand: false,
			},
		},
	}

	// Check each test case
	for _, tc := range testData {
		args := tc.inputArgs
		eb := tc.b

		b := ParseArgs(args)

		// Make sure the args match
		if !StrsEquals(args, b.Args) {
			t.Errorf("Args are wrong")
		}

		// Make sure we parsed the output properly
		if eb.Output != b.Output {
			t.Errorf("Output path wrong")
		}

		if eb.Oindex != b.Oindex {
			t.Errorf("Output index wrong")
		}

		// Now lets do the input
		if eb.Iindex != b.Iindex {
			t.Errorf("Input index wrong")
		}

		if "data/main.c" != b.Args[b.Iindex] {
			t.Errorf("Input path wrong")
		}

		// Now lets test the link command is properly recognized
		if eb.LinkCommand != b.LinkCommand {
			t.Errorf("Should not be b a link command")
		}
	}
}

func TestTempFile(t *testing.T) {
	f, err := TempFile("", "cbd-test-", ".test")

	if err != nil {
		t.Errorf("Error:", err)
	}

	name := f.Name()

	defer os.Remove(name)

	// Now lets check the file
	prefix := filepath.Join(os.TempDir(), "cbd-test-")
	suffix := ".test"

	if !strings.HasPrefix(name, prefix) {
		t.Errorf("Error '%s' does not have prefix: '%s'", name, prefix)
	}

	if !strings.HasSuffix(name, suffix) {
		t.Errorf("Error '%s' does not have suffix: '%s'", name, suffix)
	}
}
