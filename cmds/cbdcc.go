package main

import (
	"fmt"
	"github.com/jlisee/cbd"
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
	b := cbd.ParseArgs(os.Args[2:])

	// Dump arguments
	// fmt.Println("INPUTS:")
	// fmt.Println("  can distribute?:", b.Distributable)
	// fmt.Printf("  output path:  %s[%d]\n", b.Output(), b.Oindex)
	// fmt.Printf("  input path:   %s[%d]\n", b.Input(), b.Iindex)

	// TODO: Add in a local compile fast past
	if b.Distributable {
		// Pre-process the file into a compile job
		job, results, err := cbd.MakeCompileJob(compiler, b)

		if err != nil {
			fmt.Print(string(results.Output))
			os.Exit(results.Return)
		}

		// See if we have a remote host defined
		cresults, err := cbd.ClientBuildJob(job)

		if err != nil || cresults.Return != 0 {
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
		results, err := cbd.RunCmd(compiler, os.Args[2:])

		if err != nil {
			fmt.Print(string(results.Output))
			os.Exit(results.Return)
		}
	}
}
