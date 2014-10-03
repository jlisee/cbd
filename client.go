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

	// Get when we start building
	start := time.Now()

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
	worker := address

	if local {
		cresults, err = job.Compile()

		// Local build so we are building things
		worker = job.Host
	}

	// Report to server if we have a connection
	if len(server) > 0 {
		stop := time.Now()

		duration := stop.Sub(start)

		errj := reportCompletion(server, worker, job, cresults, duration)

		if err != nil {
			log.Print("Report job error: ", errj)
		}
	}

	return cresults, err
}

// findWorker uses a central server to find the desired worker
func findWorker(address string) (worker string, err error) {
	DebugPrint("Finding worker server: ", address)

	// Connect to server
	mc, err := NewTCPMessageConn(address, time.Duration(10)*time.Second)

	if err != nil {
		return
	}

	DebugPrint("  Connected")

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

	DebugPrint("Using worker: ", worker)

	return worker, nil
}

// Reports the completion of the given job to the server
func reportCompletion(address string, worker string, j CompileJob, r CompileResult, d time.Duration) error {

	jc := CompletedJob{
		Client:      j.Host,
		Worker:      worker,
		InputSize:   len(j.Input),
		OutputSize:  len(r.ObjectCode),
		CompileTime: d,
	}

	jc.computeCompileSpeed()

	// Connect to server (short timeout here so we don't hold up the build)
	mc, err := NewTCPMessageConn(address, time.Duration(1)*time.Second)

	if err != nil {
		return err
	}

	// Send completion
	err = mc.Send(jc)

	return err
}

// Build the given job on the remote host
func buildRemote(address string, job CompileJob) (CompileResult, error) {
	DebugPrint("Building on worker: ", address)

	var result CompileResult

	// Connect to the remote host so we can have it build our file
	mc, err := NewTCPMessageConn(address, time.Duration(10)*time.Second)

	if err != nil {
		return result, err
	}

	DebugPrint("  Connected")

	// Send the build job
	mc.Send(job)

	// Read back our result
	result, err = mc.ReadCompileResult()

	if err != nil {
		return result, err
	}

	DebugPrint("Build complete")

	return result, nil
}
