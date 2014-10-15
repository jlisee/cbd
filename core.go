package cbd

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	// Port used for network communications
	DefaultPort = uint(15796)
	// Whether or not we have debug logging on
	DebugLogging = false
)

type Build struct {
	Args          []string // Command line arguments
	Oindex        int      // Index of argument *before* the output file
	Iindex        int      // Index of input file option
	Cindex        int      // Index of "type" flag
	Distributable bool     // A job we can distribute
	// TODO: file type
}

// A job to be farmed out to our cluster
type CompileJob struct {
	Host     string // The host requesting it
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
	if b.Oindex < 0 || b.Oindex >= len(b.Args) {
		return ""
	}

	return b.Args[b.Oindex]
}

// Return the input path for the build job
func (b Build) Input() string {
	if b.Iindex < 0 || b.Iindex >= len(b.Args) {
		return ""
	}

	return b.Args[b.Iindex]
}

func ParseArgs(args []string) Build {
	distributable := false

	outputIndex := -1
	inputIndex := -1
	cmdIndex := -1

	for i, arg := range args {
		//idx := i + 1
		if arg == "-c" {
			distributable = true
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
		Args:          args,
		Oindex:        outputIndex,
		Iindex:        inputIndex,
		Cindex:        cmdIndex,
		Distributable: distributable,
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
	tempFile, err := TempFile(tempFileDir(), "cbd-pre-", ext)
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
	tempFile, err := TempFile(tempFileDir(), "cbd-comp-", ext)
	tempPath := tempFile.Name()

	// Make sure we return the result path if it's created
	if _, err := os.Stat(tempPath); err == nil {
		resultPath = tempPath
	}

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
		return resultPath, result, err
	}

	return resultPath, result, err
}

// MakeCompileJob takes the requested Build, pre-processses the needed
// file and returns a CompileJob with code.
func MakeCompileJob(compiler string, b Build) (j CompileJob, results ExecResult, err error) {
	// Grab hostname
	hostname, err := os.Hostname()

	if err != nil {
		return j, results, err
	}

	// Preprocess the file
	tempPreprocess, results, err := Preprocess(compiler, b)

	if len(tempPreprocess) > 0 {
		defer os.Remove(tempPreprocess)
	}

	if err != nil {
		return j, results, err
	}

	// Read file back
	preData, err := ioutil.ReadFile(tempPreprocess)

	if err != nil {
		return j, results, err
	}

	// Turn into a compile job
	j = CompileJob{
		Build:    b,
		Input:    preData,
		Compiler: compiler,
		Host:     hostname,
	}

	return j, results, nil
}

// Compile a job locally using temporary files and return the result
func (c CompileJob) Compile() (result CompileResult, err error) {
	// Open our temporary file
	ext := filepath.Ext(c.Build.Input())

	result.Return = -1

	tempFile, err := TempFile(tempFileDir(), "cbd-input-", ext)
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

	// Make sure to remove the output file if it exists
	if _, err := os.Stat(outputPath); err == nil {
		defer os.Remove(outputPath)
	}

	// Return error
	if err != nil {
		return
	}

	// Read back the code
	result.ObjectCode, err = ioutil.ReadFile(outputPath)

	return
}

// tempFileDir finds the most efficient temporary file directory on the platform
func tempFileDir() string {
	// Preferred Linux directory
	// TODO: make this cross platform and check that device has the needed space
	// before using it
	dir := "/dev/shm"

	if _, err := os.Stat(dir); err == nil {
		return dir
	} else {
		return "/tmp"
	}
}
