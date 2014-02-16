package cbuildd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

type Build struct {
	Args        []string // Command line arguments
	Output      string   // Output argument
	Oindex      int      // Index of argument *before* the output file
	Iindex      int      // Index of input file option
	Cindex      int      // Index of "type" flag
	LinkCommand bool
	// TODO: file type
}

func ParseArgs(args []string) Build {
	nolink := false

	var output string
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
			output = args[outputIndex]
		} else if (arg[0] != '-') && (outputIndex != i) {
			// For now assume any non flag argument, not the -o target
			// is our Build flag
			inputIndex = i
		}
	}

	b := Build{
		Args:        args,
		Output:      output,
		Oindex:      outputIndex,
		Iindex:      inputIndex,
		Cindex:      cmdIndex,
		LinkCommand: !nolink,
	}

	return b
}

// Build the file at the temporary location, you must clean up the returned
// file.
func Preprocess(b Build) (string, error) {
	// Lets create a temporary file
	tempFile, err := ioutil.TempFile(os.TempDir(), "cbuildd-pre-")
	tempPath := tempFile.Name()

	if err != nil {
		return "", err
	}

	// Update the arguments to adjust the output path
	gccArgs := make([]string, len(b.Args))
	copy(gccArgs, b.Args)
	gccArgs[b.Oindex] = tempPath

	// Remove change the "-c" into a "-E"
	gccArgs[b.Cindex] = "-E"

	// Run gcc with the rest of our args
	err = RunCmd("gcc", gccArgs)

	if err != nil {
		return "", err
	}

	return tempPath, err
}

// Build the file at the temporary location, you must clean up the returned
// file.
func Compile(b Build, input string) (string, error) {
	// Lets create a temporary file
	tempFile, err := ioutil.TempFile(os.TempDir(), "cbuildd-comp-")
	tempPath := tempFile.Name()

	if err != nil {
		return "", err
	}

	// Update the arguments to point the output path to the temp directory and
	// the input path from the given location
	gccArgs := make([]string, len(b.Args)+2)
	copy(gccArgs[2:], b.Args)

	gccArgs[b.Oindex+2] = tempPath

	if len(input) > 0 {
		gccArgs[b.Iindex+2] = input
	}

	// We need to manual specify the language because are temp
	// file doesn't have the proper extension (TODO: don't assume c)
	gccArgs[0] = "-x"
	gccArgs[1] = "c"

	// Run gcc with the rest of our args
	// TODO: always include error output no matter what, needed for debugging
	err = RunCmd("gcc", gccArgs)

	if err != nil {
		return "", err
	}

	return tempPath, err
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

// Executes, returning the stdout if the program fails (the return code is
// ignored)
func RunCmd(prog string, args []string) error {
	fmt.Printf("Run: %s ", prog)
	for _, arg := range args {
		fmt.Printf("%s ", arg)
	}
	fmt.Println()

	cmd := exec.Command(prog, args...)

	//var out bytes.Buffer
	var outErr bytes.Buffer
	//cmd.Stdout = &out
	cmd.Stderr = &outErr

	err := cmd.Run()
	//fmt.Printf("OUTPUT: %q\n", out.String())

	if err != nil {
		return errors.New(outErr.String())
	}

	return nil
}
