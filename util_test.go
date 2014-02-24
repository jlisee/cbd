package cbd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
			Output: []byte("go version go1.2 linux/amd64\n"),
			Return: 0,
		},
		"go bob": ExecResult{
			Output: []byte("go: unknown subcommand \"bob\"\nRun 'go help' for usage.\n"),
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

		if 0 != bytes.Compare(res.Output, eres.Output) {
			t.Errorf("Got output: %s instead of: %s", string(res.Output),
				string(eres.Output))
		}
	}
}
