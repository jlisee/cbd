package main

import (
	"fmt"
	"github.com/jlisee/cbuildd"
	"log"
	"os"
)

func main() {
	// Pull of the first argument and make it our compiler (later make this a
	// just a configuration setting)
	compiler := os.Args[1]

	// We have to parse the arguments manually because the default flag package
	// stops parsing after positional args, and github.com/ogier/pflag errors out
	// on unknown arguments.

	b := cbuildd.ParseArgs(os.Args[2:])

	// Dump arguments
	fmt.Println("INPUTS:")
	fmt.Println("  link command?:", b.LinkCommand)
	fmt.Printf("  output path:  %s[%d]\n", b.Args[b.Oindex], b.Oindex)
	fmt.Printf("  input path:   %s[%d]\n", b.Args[b.Iindex], b.Iindex)

	if !b.LinkCommand {
		// Pre-process
		tempPreprocess, err := cbuildd.Preprocess(compiler, b)

		if len(tempPreprocess) > 0 {
			defer os.Remove(tempPreprocess)
		}

		if err != nil {
			log.Fatal(err)
		}

		// Lets compile things
		tempOutput, err := cbuildd.Compile(compiler, b, tempPreprocess)

		if len(tempOutput) > 0 {
			defer os.Remove(tempOutput)
		}

		if err != nil {
			log.Fatal(err)
		}

		// Copy the file to the resulting location
		err = os.Rename(tempOutput, b.Output())
		if err != nil {
			// Can't use the efficient rename, so lets us the copy
			err = cbuildd.Copyfile(b.Output(), tempOutput)
		}

		if err != nil {
			log.Fatal(err)
		}
	} else {
		err := cbuildd.RunCmd(compiler, os.Args[2:])

		if err != nil {
			log.Fatal(err)
		}
	}
}
