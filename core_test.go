package cbuildd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

type ParseTestCase struct {
	inputArgs []string
	b         Build // Resulting build object
}

func TestParseArgs(t *testing.T) {
	// Note args is left out of the Build struct because it supplied separately
	testData := []ParseTestCase{
		ParseTestCase{
			inputArgs: []string{"-c", "data/main.c", "-o", "main.o"},
			b: Build{
				Oindex:      3,
				Iindex:      1,
				Cindex:      0,
				LinkCommand: false,
			},
		},
	}

	// Check each test case
	for _, tc := range testData {
		// Make sure to set the args for the test case
		args := tc.inputArgs
		eb := tc.b
		eb.Args = args

		b := ParseArgs(args)

		// Make sure the args match
		if !StrsEquals(args, b.Args) {
			t.Errorf("Args are wrong")
		}

		// Make sure we parsed the output properly
		if eb.Output() != b.Output() {
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

// Put in a test for RunCmd here, make sure we are getting back stderr and stdout
func TestRunCmd(t *testing.T) {
	tests := map[string]ExecResult{
		"go version": ExecResult{
			Output: bytes.NewBufferString("go version go1.2 linux/amd64\n"),
			Return: 0,
		},
		"go bob": ExecResult{
			Output: bytes.NewBufferString("go: unknown subcommand \"bob\"\nRun 'go help' for usage.\n"),
			Return: 2,
		},
	}

	for cmd, eres := range tests {
		// Split up string to get the command and it's args
		args := strings.Split(cmd, " ")

		res, err := RunCmd(args[0], args[1:])

		// Ignore the errors that occur with non-zero return codes
		if eres.Return == 0 {
			if err != nil {
				t.Errorf("Run command failed with: %s", err)
			}
		}

		// Now check our results
		if res.Return != eres.Return {
			t.Errorf("Got return: %d instead of: %d", eres.Return, res.Return)
		}

		if res.Output.String() != eres.Output.String() {
			t.Errorf("Got output: %s instead of: %s", res.Output.String(),
				eres.Output.String())
		}
	}
}
