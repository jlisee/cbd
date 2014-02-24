package cbd

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	// Port used for network communications
	Port = 15796
)

type Build struct {
	Args        []string // Command line arguments
	Oindex      int      // Index of argument *before* the output file
	Iindex      int      // Index of input file option
	Cindex      int      // Index of "type" flag
	LinkCommand bool
	// TODO: file type
}

// A job to be farmed out to our cluster
type CompileJob struct {
	Build    Build  // The commands to build it with
	Input    []byte // The data to build
	Compiler string // The compiler to run it with
}

// The result of a compile
type CompileResult struct {
	ExecResult        // Results of the compiler command
	ObjectCode []byte // The compiled object code
}

// Returns the output path build job
func (b Build) Output() string {
	return b.Args[b.Oindex]
}

// Return the input path for the build job
func (b Build) Input() string {
	return b.Args[b.Iindex]
}

func ParseArgs(args []string) Build {
	nolink := false

	var outputIndex int
	var inputIndex int
	var cmdIndex int

	for i, arg := range args {
		//idx := i + 1
		if arg == "-c" {
			nolink = true
			cmdIndex = i
		}
		if arg == "-o" {
			outputIndex = i + 1
		} else if (arg[0] != '-') && (outputIndex != i) {
			// For now assume any non flag argument, not the -o target
			// is our Build flag
			inputIndex = i
		}
	}

	b := Build{
		Args:        args,
		Oindex:      outputIndex,
		Iindex:      inputIndex,
		Cindex:      cmdIndex,
		LinkCommand: !nolink,
	}

	return b
}

// Build the file at the temporary location, you must clean up the returned
// file.
func Preprocess(compiler string, b Build) (resultPath string, result ExecResult, err error) {
	// Set a default return code
	result.Return = -1

	// Get the extension of the output file
	ext := filepath.Ext(b.Input())

	// Lets create a temporary file
	tempFile, err := TempFile("", "cbd-comp-", ext)
	tempPath := tempFile.Name()

	if err != nil {
		return
	}

	// Update the arguments to adjust the output path
	gccArgs := make([]string, len(b.Args))
	copy(gccArgs, b.Args)
	gccArgs[b.Oindex] = tempPath

	// Remove change the "-c" into a "-E"
	gccArgs[b.Cindex] = "-E"

	// Run gcc with the rest of our args
	result, err = RunCmd(compiler, gccArgs)

	if err != nil {
		return "", result, err
	}

	return tempPath, result, err
}

// Build the file at the temporary location, you must clean up the returned
// file.
func Compile(compiler string, b Build, input string) (resultPath string, result ExecResult, err error) {
	// Set a default return code
	result.Return = -1

	// Get the extension of the output file
	ext := filepath.Ext(b.Output())

	// Lets create a temporary file
	tempFile, err := TempFile("", "cbd-comp-", ext)
	tempPath := tempFile.Name()

	if err != nil {
		return
	}

	// Update the arguments to point the output path to the temp directory and
	// the input path from the given location
	gccArgs := make([]string, len(b.Args))

	copy(gccArgs, b.Args)

	gccArgs[b.Oindex] = tempPath
	gccArgs[b.Iindex] = input

	// Run gcc with the rest of our args
	// TODO: always include error output no matter what, needed for debugging
	result, err = RunCmd(compiler, gccArgs)

	if err != nil {
		return "", result, err
	}

	return tempPath, result, err
}

// Compile a job locally using temporary files and return the result
func (c CompileJob) Compile() (result CompileResult, err error) {
	// Open our temporary file
	ext := filepath.Ext(c.Build.Input())

	result.Return = -1

	tempFile, err := TempFile("", "cbd-comp-", ext)
	tempPath := tempFile.Name()

	if err != nil {
		return
	}

	defer os.Remove(tempPath)

	// Write our output to our temporary file
	tempFile.Write(c.Input)

	// Build everything
	outputPath, compileResult, err := Compile(c.Compiler, c.Build, tempPath)

	result.ExecResult = compileResult

	if err != nil {
		return
	}

	defer os.Remove(outputPath)

	// Read back the code
	result.ObjectCode, err = ioutil.ReadFile(outputPath)

	return
}
