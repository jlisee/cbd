package main

import (
	"fmt"
	"github.com/jlisee/cbuildd"
	"io/ioutil"
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
		tempPreprocess, results, err := cbuildd.Preprocess(compiler, b)

		if len(tempPreprocess) > 0 {
			defer os.Remove(tempPreprocess)
		}

		if err != nil {
			fmt.Print(string(results.Output))
			os.Exit(results.Return)
		}

		// Read file back
		preData, err := ioutil.ReadFile(tempPreprocess)

		if err != nil {
			log.Fatal(err)
		}

		// Turn into a compile job
		job := cbuildd.CompileJob{
			Build: b,
			Input: preData,
			Compiler: compiler,
		}

		// Build it! (todo: make this remote)
		cresults, err := job.Compile()

		if err != nil {
			fmt.Print(string(cresults.Output))
			os.Exit(cresults.Return)
		}

		// Now write the results to right output location
		f, err := os.Create(b.Output())

		if err != nil {
			log.Fatal(err)
		}

		defer f.Close()

		_, err = f.Write(cresults.ObjectCode)

		if err != nil {
			log.Fatal(err)
		}

	} else {
		results, err := cbuildd.RunCmd(compiler, os.Args[2:])

		if err != nil {
			fmt.Print(string(results.Output))
			os.Exit(results.Return)
		}
	}
}

// // Build the given job on the remote host
// func buildRemote(b Build, prepath string, host string) {
// 	// Connect to the remote host so we can have it build our file

// 	// Create encoders so we can send our data across the wire
// 	enc := gob.NewEncoder(&conn) // Will write to network.
// 	dec := gob.NewDecoder(&conn) // Will read from network.

// 	// Send the build job

// 	// Wait for the file to come back
// }
