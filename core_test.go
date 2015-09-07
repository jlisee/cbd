package cbd

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
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
		// Base case
		ParseTestCase{
			inputArgs: []string{"-c", "data/main.c", "-o", "main.o"},
			b: Build{
				Oindex:        3,
				Iindex:        1,
				Cindex:        0,
				IgnoreIndex:   []int{},
				Distributable: true,
			},
		},
		// No recognized args
		ParseTestCase{
			inputArgs: []string{"-dumpversion"},
			b: Build{
				Oindex:        -1,
				Iindex:        -1,
				Cindex:        -1,
				IgnoreIndex:   []int{},
				Distributable: false,
			},
		},
		// Dependency generation
		ParseTestCase{
			inputArgs: []string{"-MMD", "-MT", "main.c.o", "-MF", "main.c.o.d", "-c", "data/main.c", "-o", "main.o"},
			b: Build{
				Oindex:        8,
				Iindex:        6,
				Cindex:        5,
				IgnoreIndex:   []int{0, 1, 2, 3, 4},
				Distributable: true,
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

		// Make sure the rest of the structure matched
		if !reflect.DeepEqual(eb, b) {
			t.Errorf("Wrong build, wanted:\n %+v got:\n %+v", eb, b)
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
		defer os.Remove(filePath)
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

func TestMakeCompileJob(t *testing.T) {
	// Create our build job
	b := ParseArgs(strings.Split("-c data/main.c -o main.o", " "))

	j, result, err := MakeCompileJob("gcc", b)

	if err != nil {
		t.Errorf("MakeCompile returned error: %s (Output: %s)", err,
			string(result.Output))
	}

	if result.Return != 0 {
		t.Errorf("Preprocess returned: %d", result.Return)
	}

	// Now grab our hostname
	hostname, err := os.Hostname()

	if err != nil {
		t.Error("Error getting hostname: ", err)
	}

	// Now check the job
	if hostname != j.Host {
		t.Errorf("Job hostname '%s' incorrect, not '%s'", j.Host, hostname)
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
	} else {
		defer os.Remove(filePath)
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
