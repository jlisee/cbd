package cbuildd

import (
	"bytes"
	"io/ioutil"
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

// This test requires gcc to be installed
func TestPreprocess(t *testing.T) {
	b := ParseArgs(strings.Split("-c data/main.c -o main.o", " "))
	filePath, result, err := Preprocess("gcc", b)

	if err != nil {
		t.Errorf("Preprocess returned error: %s (Output: %s)", err,
			string(result.Output))
	}

	if result.Return != 0 {
		t.Errorf("Preprocess returned: %d", result.Return)
	}

	// Make sure we have the right extension
	ext := filepath.Ext(filePath)
	if ext != ".c" {
		t.Error("File does not have required .c extension has:", ext)
	}

	// Make sure the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Output file does not exist:", filePath)
		return
	} else {
		//defer os.Remove(filePath)
	}

	// Makes sure the file contains C source code
	contents, err := ioutil.ReadFile(filePath)

	if err != nil {
		t.Error("Could not read file:", err)
	}

	if !bytes.Contains(contents, []byte("printf(\"Hello, world!\\n\");")) {
		t.Error("Output didn't contain C code:", string(contents))
	}
}

// This test requires gcc to be installed
func TestCompile(t *testing.T) {
	// Create a temporary file and copy the C source code into that location
	f, err := TempFile("", "cbd-test-", ".c")
	tempFile := f.Name()

	defer os.Remove(tempFile)

	Copyfile(tempFile, "data/main.c")

	// Now lets build that temp code
	b := ParseArgs(strings.Split("-c data/nothere.c -o main.o", " "))
	filePath, result, err := Compile("gcc", b, tempFile)

	if err != nil {
		t.Errorf("Compile returned error: %s (Output: %s)", err,
			string(result.Output))
	}

	if result.Return != 0 {
		t.Errorf("Compile returned: %d", result.Return)
	}

	// Make sure we have the right extension
	ext := filepath.Ext(filePath)
	if ext != ".o" {
		t.Error("File does not have required .o extension has:", ext)
	}

	// Make sure the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Output file does not exist:", filePath)
		return
	}

	// TODO: Make sure the file contains object code
}

func TestCompileJobCompile(t *testing.T) {

	// Build and run our compile job
	tests := map[string]CompileJob{
		"": CompileJob{
			Build:    ParseArgs(strings.Split("-c data/main.c -o main.o", " ")),
			Input:    []byte("#include <stdio.h>\nint main() { printf(\"Hello, world!\\n\");  return 0; } "),
			Compiler: "gcc",
		},
		"error: expected ‘;’ before ‘return’": CompileJob{
			Build:    ParseArgs(strings.Split("-c data/main.c -o main.o", " ")),
			Input:    []byte("#include <stdio.h>\nint main() { printf(\"Hello, world!\\n\")  return 0; } "),
			Compiler: "gcc",
		},
	}

	for output, job := range tests {

		result, err := job.Compile()

		// Set the return code based on output
		inError := false
		eret := 0
		if len(output) > 0 {
			eret = 1
			inError = true
		}

		if !inError && err != nil {
			t.Error("Error with compiling job:", err)
		}

		// Test the return code
		if result.Return != eret {
			t.Errorf("Compile returned: %d", result.Return)
		}

		// Make sure we have actual code back
		if !inError && len(result.ObjectCode) == 0 {
			t.Errorf("Compile return no output data")
		}

		// Make sure we have no error text
		if inError {
			if !strings.Contains(string(result.Output), output) {
				t.Errorf("Compile output: '%s' does not contain '%s'",
					string(result.Output), output)
			}
		} else {
			if 0 != len(result.Output) {
				t.Errorf("Compile had output: '%s'", string(result.Output))
			}
		}
	}
}
