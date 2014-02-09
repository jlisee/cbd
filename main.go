package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

type Build struct {
	args   []string // Command line arguments
	oIndex int // Index of output file
	iIndex int // Index of input file
	cIndex int // Index of "type" flag
	// TODO: file type
}

func main() {
	// We have to parse the arguments manually because the default flag package
	// stops parsing after positional args, and github.com/ogier/pflag errors out
	// on unknown arguments.

	nolink := false
	var output string
	var input string
	var outputIndex int
	var inputIndex int
	var cmdIndex int

	for i, arg := range os.Args[1:] {
		//idx := i + 1
		fmt.Printf("  %d: %s\n", i, arg)

		if arg == "-c" {
			nolink = true
			cmdIndex = i
		}
		if arg == "-o" {
			outputIndex = i + 1
			output = os.Args[outputIndex + 1]
		} else if (arg[0] != '-') && (outputIndex != i) {
			// For now assume any non flag argument, not the -o target
			// is our Build flag
			input = arg
			inputIndex = i
		}
	}

	// Dump arguments
	fmt.Println("INPUTS:")
	fmt.Println("  link command?:", !nolink)
	fmt.Printf("  output path:  %s[%d]\n", output, outputIndex)
	fmt.Printf("  input path:   %s[%d]\n", input, inputIndex)

	if nolink {
		b := Build{
			args: os.Args[1:],
			oIndex: outputIndex,
			iIndex: inputIndex,
			cIndex: cmdIndex,
		}

		// Pre-process
		tempPreprocess, err := Preprocess(b)

		if len(tempPreprocess) > 0 {
			defer os.Remove(tempPreprocess)
		}

		if err !=  nil {
			log.Fatal(err)
		}

		// Lets compile things
		tempOutput, err := Compile(b, tempPreprocess)

		if len(tempOutput) > 0 {
			defer os.Remove(tempOutput)
		}

		if err !=  nil {
			log.Fatal(err)
		}

		// Copy the file to the resulting location
		err = os.Rename(tempOutput, output)
		if err != nil {
			// Can't use the efficient rename, so lets us the copy
			err = copyfile(output, tempOutput)
		}

		if err !=  nil {
			log.Fatal(err)
		}
	} else {
		err := RunCmd("gcc", os.Args[1:])

		if err !=  nil {
			log.Fatal(err)
		}
	}
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
	gccArgs := make([]string, len(b.args))
	copy(gccArgs, b.args)
	gccArgs[b.oIndex] = tempPath

	// Remove change the "-c" into a "-E"
	gccArgs[b.cIndex] = "-E"

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
	gccArgs := make([]string, len(b.args) + 2)
	copy(gccArgs[2:], b.args)

	gccArgs[b.oIndex + 2] = tempPath

	if len(input) > 0 {
		gccArgs[b.iIndex + 2] = input
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
func copyfile(dst, src string) error {
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
	for _, arg := range(args) {
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


