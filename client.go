// This file contains all the routines used by the client process that stands
// in for the compiler, and farms jobs out to workers.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// TODO: this needs some tests
func ClientBuildJob(job CompileJob) (cresults CompileResult, err error) {
	address := os.Getenv("CBD_POTENTIAL_HOST")
	server := os.Getenv("CBD_SERVER")
	local := false

	// Grab our ID
	id, err := GetMachineID()

	if err != nil {
		log.Print("Failed to get the local machine ID: ", err)
	}

	var worker MachineName

	// If we have a server, but no hosts, go with the server
	if len(address) == 0 && len(server) > 0 {
		server = addPortIfNeeded(server, DefaultServerPort)

		address, worker, err = findWorker(server, id)

		if err != nil {
			log.Print("Find worker error: ", err)
		}
	}

	// Get when we start building
	start := time.Now()

	// Try to build on the remote host if we have found one
	if len(address) > 0 {
		address = addPortIfNeeded(address, DefaultWorkerPort)
		cresults, err = buildRemote(address, job)

		// If the remote build failed switch to local
		if err != nil {
			log.Print("Remote build error: ", err)
			local = true
		}
	} else {
		local = true
	}

	// Disable local builds when in our special test mode
	no_local := os.Getenv("CBD_NO_LOCAL")

	if local && no_local == "yes" {
		return cresults, fmt.Errorf("Can't find worker")
	}

	// Determine our local id
	ln := MachineName{
		ID:   id,
		Host: job.Host,
	}

	// Build it locally if all else has failed
	if local {
		cresults, err = job.Compile()

		// Local build so we are building things
		worker = ln
	}

	// Report to server if we have a connection
	if len(server) > 0 {
		stop := time.Now()

		duration := stop.Sub(start)

		errj := reportCompletion(server, ln, worker, job, cresults, duration)

		if errj != nil {
			log.Print("Report job error: ", errj)
		}
	}

	return
}

// findWorker uses a central server to find the desired worker
func findWorker(server string, id MachineID) (address string, worker MachineName, err error) {
	DebugPrint("Finding worker server: ", server)

	// Set a timeout for this entire process and just build locally
	quittime := time.Now().Add(time.Duration(10) * time.Second)

	// Connect to server
	mc, err := NewTCPMessageConn(server, time.Duration(10)*time.Second)

	if err != nil {
		return
	}

	DebugPrint("  Connected")

	// Get hostname
	hostname, err := os.Hostname()

	if err != nil {
		return
	}

	// Get IP addresses on the machine
	addrs, err := getLocalIPAddrs()

	if err != nil {
		return
	}

	// Send our request
	rq := WorkerRequest{
		Client: MachineName{
			ID:   id,
			Host: hostname,
		},
		Addrs: addrs,
	}
	mc.Send(rq)

	// Wait until we get a valid worker response, or we timeout
	var r WorkerResponse
Loop:
	for {
		// Read back the latest from the server
		r, err = mc.ReadWorkerResponse()

		// If there is an error talking bail out
		if err != nil {
			return
		}

		// Handle our response types
		switch r.Type {
		case Queued:
			// Timeout if it's been too long
			if time.Now().After(quittime) {
				err = fmt.Errorf("Timed out waiting for a free worker")
				return
			} else {
				DebugPrint("No workers available waiting...")
			}
		case NoWorkers:
			// No workers present in cluster bail out
			err = fmt.Errorf("No workers in cluster")
			return
		case Valid:
			// We have useful information break out of the loop
			break Loop
		default:
			// We don't understand this response type
			err = fmt.Errorf("Unknown response type: ", r.Type)
			return
		}
	}

	address = r.Address.IP.String() + ":" + strconv.Itoa(r.Port)
	worker = MachineName{
		ID:   r.ID,
		Host: r.Host,
	}

	DebugPrintf("Using worker: %s (%s)", r.Host, address)

	return address, worker, nil
}

// Reports the completion of the given job to the server
func reportCompletion(address string, c MachineName, w MachineName, j CompileJob, r CompileResult, d time.Duration) error {

	jc := CompletedJob{
		Client:      c,
		Worker:      w,
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
