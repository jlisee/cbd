package cbd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
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
	// Dynamically build the "version GOOS/GOARCH" string that go produces
	verstr := runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH

	tests := map[string]ExecResult{
		"go version": ExecResult{
			Output: []byte("go version " + verstr + "\n"),
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

func TestGetLoadAverage(t *testing.T) {
	load, err := GetLoadAverage()

	if err != nil {
		t.Errorf("Error:", err)
		return
	}

	if load <= 0 {
		t.Error("Load bad: ", load)
	}
}

func TestGUID(t *testing.T) {
	g1 := NewGUID()
	g2 := NewGUID()

	if g1 == g2 {
		t.Error("GUIDs match but should be different: ", g1, g2)
	}

	s1 := g1.String()
	s2 := g2.String()

	if s1 == s2 {
		t.Error("GUIDs match but should be different: ", s1, s2)
	}
}
