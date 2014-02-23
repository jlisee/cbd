package cbuildd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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

// The result of running a command
type ExecResult struct {
	Output []byte // Output of the command
	Return int    // Return code of program
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
	tempFile, err := TempFile("", "cbuildd-comp-", ext)
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
	tempFile, err := TempFile("", "cbuildd-comp-", ext)
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

// copies dst to src location, no metadata is copied
func Copyfile(dst, src string) error {
	s, err := os.Open(src)

	if err != nil {
		return err
	}

	// No need to check errors on read only file, we already got everything
	// we need from the filesystem, so nothing can go wrong now.
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

// Compile a job locally using temporary files and return the result
func (c CompileJob) Compile() (result CompileResult, err error) {
	// Open our temporary file
	ext := filepath.Ext(c.Build.Input())

	result.Return = -1

	tempFile, err := TempFile("", "cbuildd-comp-", ext)
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

// Executes, returning the stdout if the program fails (the return code is
// ignored)
func RunCmd(prog string, args []string) (result ExecResult, err error) {
	fmt.Printf("Run: %s ", prog)
	for _, arg := range args {
		fmt.Printf("%s ", arg)
	}
	fmt.Println()

	cmd := exec.Command(prog, args...)

	// Setup the buffer to hold the output
	// TODO: consider caching this buffer
	buffer := new(bytes.Buffer)

	cmd.Stdout = buffer
	cmd.Stderr = buffer

	err = cmd.Run()

	// Copy over our buffer
	result.Output = buffer.Bytes()

	// Get the return code out of the error
	if err != nil {
		result.Return = -1

		// Possibly Linux specific example
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				result.Return = status.ExitStatus()
			}
		}
	}

	return
}

// Generates and opens a temporary file with a defined prefix and suffix
// This is the same api as ioutil.TempFile accept it accepts a suffix
//  TODO: see if this is too slow
func TempFile(dir, prefix string, suffix string) (f *os.File, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	// The maximum size of random file count
	// TODO: see if we can do this at package scope somehow
	var maxRand *big.Int = big.NewInt(0)
	maxRand.SetString("FFFFFFFFFFFFFFFF", 16)

	var randNum *big.Int

	for i := 0; i < 10000; i++ {
		// Generate random part of the path name
		randNum, err = rand.Int(rand.Reader, maxRand)

		if err != nil {
			return
		}

		// Transform to an int
		randString := hex.EncodeToString(randNum.Bytes())

		// Attempt to open file and fail if it already exists
		name := filepath.Join(dir, prefix+randString+suffix)
		f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		break
	}
	return
}
