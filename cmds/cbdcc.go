package main

import (
	"fmt"
	"github.com/jlisee/cbd"
	"log"
	"os"
)

var ()

func main() {
	// Pull of the first argument and make it our compiler (later make this a
	// just a configuration setting)
	compiler := os.Args[1]

	// We have to parse the arguments manually because the default flag package
	// stops parsing after positional args, and github.com/ogier/pflag errors out
	// on unknown arguments.
	b := cbd.ParseArgs(os.Args[2:])

	// Setup logging if needed
	logpath := os.Getenv("CBD_LOGFILE")

	if len(logpath) > 0 {
		// Open the log file for appending
		f, err := os.OpenFile(logpath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)

		if err != nil {
			log.Fatal(err)
		}

		defer f.Close()

		log.SetOutput(f)
		log.Print("ARGS: ", os.Args[2:])
		log.Printf("  Distribute?: %t", b.Distributable)
		log.Printf("  Output path:  %s[%d]\n", b.Output(), b.Oindex)
		log.Printf("  Input path:   %s[%d]\n", b.Input(), b.Iindex)

		cbd.DebugLogging = true
	}
	// Dump arguments

	// TODO: Add in a local compile fast past
	if b.Distributable {
		// Pre-process the file into a compile job
		job, results, err := cbd.MakeCompileJob(compiler, b)

		if err != nil {
			fmt.Print(string(results.Output))
			cbd.DebugPrint("Preprocess Error: ", string(results.Output))
			os.Exit(results.Return)
		}

		// See if we have a remote host defined
		cresults, err := cbd.ClientBuildJob(job)

		if err != nil || cresults.Return != 0 {
			fmt.Print(string(cresults.Output))
			cbd.DebugPrint("Build Error: ", string(cresults.Output))
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

		cbd.DebugPrint("Remote Success")

	} else {
		results, err := cbd.RunCmd(compiler, os.Args[2:])

		if err != nil {
			fmt.Print(string(results.Output))
			cbd.DebugPrint("Local Error: ", string(results.Output))
			os.Exit(results.Return)
		}

		cbd.DebugPrint("Success")
	}
}
