package main

import (
	"fmt"
	"log"
	"os"
	"github.com/jlisee/cbuildd"
)

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
		b := cbuildd.Build{
			Args: os.Args[1:],
			Oindex: outputIndex,
			Iindex: inputIndex,
			Cindex: cmdIndex,
		}

		// Pre-process
		tempPreprocess, err := cbuildd.Preprocess(b)

		if len(tempPreprocess) > 0 {
			defer os.Remove(tempPreprocess)
		}

		if err !=  nil {
			log.Fatal(err)
		}

		// Lets compile things
		tempOutput, err := cbuildd.Compile(b, tempPreprocess)

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
			err = cbuildd.Copyfile(output, tempOutput)
		}

		if err !=  nil {
			log.Fatal(err)
		}
	} else {
		err := cbuildd.RunCmd("gcc", os.Args[1:])

		if err !=  nil {
			log.Fatal(err)
		}
	}
}
