package main

import (
	"fmt"
	"github.com/jlisee/cbd"
	"io/ioutil"
	"log"
	"os"
	"strconv"
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
		cresults, err := buildJob(job)

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

func buildJob(job cbd.CompileJob) (cresults cbd.CompileResult, err error) {
	address := os.Getenv("CBD_POTENTIAL_HOST")
	server := os.Getenv("CBD_SERVER")
	local := false

	// If we have a server, but no hosts, go with the server
	if len(address) == 0 && len(server) > 0 {
		address, err = findWorker(server)

		if err != nil {
			log.Print("Find worker error: ", err)
		}
	}

	// Try to build on the remote host if we have found one
	if len(address) > 0 {
		cresults, err = buildRemote(address, job)

		// If the remote build failed switch to local
		if err != nil {
			log.Print("Remote build error: ", err)
			local = true
		}
	} else {
		local = true
	}

	// Build it locally if all else has failed
	if local {
		cresults, err = job.Compile()
	}

	return cresults, err
}

// findWorker uses a central server to find the desired worker
func findWorker(address string) (worker string, err error) {
	// Connect to server
	mc, err := cbd.NewTCPMessageConn(address, time.Duration(10)*time.Second)

	// Get hostname
	hostname, err := os.Hostname()

	if err != nil {
		return
	}

	// Send our request
	rq := cbd.WorkerRequest{
		Client: hostname,
	}
	mc.Send(rq)

	// Read back our response
	r, err := mc.ReadWorkerResponse()

	if err != nil {
		return
	}

	worker = r.Worker + ":" + strconv.Itoa(r.Port)
	return worker, nil
}

// Build the given job on the remote host
func buildRemote(address string, job cbd.CompileJob) (cbd.CompileResult, error) {
	var result cbd.CompileResult

	// Connect to the remote host so we can have it build our file
	mc, err := cbd.NewTCPMessageConn(address, time.Duration(10)*time.Second)

	if err != nil {
		return result, err
	}

	// Send the build job
	mc.Send(job)

	// Read back our result
	result, err = mc.ReadCompileResult()

	if err != nil {
		return result, err
	}

	return result, nil
}
