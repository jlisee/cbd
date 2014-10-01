// This file contains all the routines used by the client process that stands
// in for the compiler, and farms jobs out to workers.

package cbd

import (
	"log"
	"os"
	"strconv"
	"time"
)

func ClientBuildJob(job CompileJob) (cresults CompileResult, err error) {
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
	mc, err := NewTCPMessageConn(address, time.Duration(10)*time.Second)

	if err != nil {
		return
	}

	// Get hostname
	hostname, err := os.Hostname()

	if err != nil {
		return
	}

	// Send our request
	rq := WorkerRequest{
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
func buildRemote(address string, job CompileJob) (CompileResult, error) {
	var result CompileResult

	// Connect to the remote host so we can have it build our file
	mc, err := NewTCPMessageConn(address, time.Duration(10)*time.Second)

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
