package main

import (
	"fmt"
	"github.com/jlisee/cbd"
	"io/ioutil"
	"log"
	"os"
	"time"
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
	fmt.Println("INPUTS:")
	fmt.Println("  link command?:", b.LinkCommand)
	fmt.Printf("  output path:  %s[%d]\n", b.Args[b.Oindex], b.Oindex)
	fmt.Printf("  input path:   %s[%d]\n", b.Args[b.Iindex], b.Iindex)

	// TODO: Add in a local compile fast past
	if !b.LinkCommand {
		// Pre-process
		tempPreprocess, results, err := cbd.Preprocess(compiler, b)

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
		job := cbd.CompileJob{
			Build:    b,
			Input:    preData,
			Compiler: compiler,
		}

		// See if we have a remote host defined
		host := os.Getenv("CBD_POTENTIAL_HOST")

		var cresults cbd.CompileResult

		if len(host) > 0 {
			// Build it remotely
			cresults, err = buildRemote(host, job)
		} else {
			// Build it locally
			cresults, err = job.Compile()
		}

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

// Build the given job on the remote host
func buildRemote(host string, job cbd.CompileJob) (cbd.CompileResult, error) {
	var result cbd.CompileResult

	// Connect to the remote host so we can have it build our file
	mc, err := cbd.NewTCPMessageConn(host, cbd.Port, time.Duration(10)*time.Second)

	// Send the build job
	mc.Send(job)

	// Read back our result
	result, err = mc.ReadCompileResult()

	if err != nil {
		return result, err
	}

	return result, nil
}
