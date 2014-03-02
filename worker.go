// This file contains all the routines used by the worker process. It
// executes CompileJobs and sends WorkerState updates to a server
// process if desired.

package cbd

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Worker struct {
	port  int    // Port we listen for connections on
	saddr string // Port of the server (if it exists)
	run   bool   // Should the update loop keep running?
}

// NewWorker initializes a Worker struct based on the given server and
// local address.  The local address will be parsed to determine our
// local port for receiving connections.
func NewWorker(laddr string, saddr string) (w *Worker, err error) {
	w = new(Worker)
	w.saddr = saddr
	w.run = true

	// Parse out our port
	parts := strings.Split(laddr, ":")

	port := Port

	if len(parts) > 1 {
		port, err = strconv.Atoi(parts[len(parts)-1])

		if err != nil {
			msg := fmt.Sprintf("Error parsing port out of \"%s\": %s", laddr, err)
			return nil, errors.New(msg)
		}
	}

	w.port = port

	return w, nil
}

// Serve listens for incoming build requests connections and spawns
// goroutines to handle them as needed.  If we have a server address
// it will send status updates there as well.
func (w *Worker) Serve(ln net.Listener) {
	// Start update goroutine if present
	if len(w.saddr) > 0 {
		go w.updateServer()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go w.handleRequest(conn)
	}
}

func (w *Worker) handleRequest(conn DeadlineReadWriter) {
	log.Print("Handling request...")

	// Decode the CompileJob
	mc := NewMessageConn(conn, time.Duration(10)*time.Second)
	job, err := mc.ReadCompileJob()

	if err != nil {
		log.Print("Decode error:", err)
		return
	}

	// Build the code
	cresults, _ := job.Compile()

	// Send back the result
	err = mc.Send(cresults)

	if err != nil {
		log.Print("Encode error:", err)
		return
	}

	log.Print("Done.")
}

// updateServer will do it's best to maintain a connection to the main
// server, and send it WorkerState updates
func (w *Worker) updateServer() {
	// How often we try to establish a connection
	interval := time.Duration(1) * time.Second

	// Get host name
	hostname, err := os.Hostname()

	if err != nil {
		log.Fatal("Could not find hostname: ", err)
	}

	for {
		// Open up
		mc, err := NewTCPMessageConn(w.saddr, time.Duration(10)*time.Second)

		if err != nil {
			log.Print("Error connecting to server: ", err)
			time.Sleep(interval)
			continue
		}

		// Send updates
		err = w.sendWorkerState(mc, hostname)

		if err != nil {
			log.Print("Error sending message to server: ", err)
			time.Sleep(interval)
			continue
		}
	}
}

// sendWorkerState sends updates to our server until the connection
// fails
func (w *Worker) sendWorkerState(mc *MessageConn, host string) error {
	// Get capacity
	capacity := runtime.NumCPU()

	for {
		// Update the state with the latest information
		ws := WorkerState{
			Host:     host,
			Port:     w.port,
			Capacity: capacity,
			Load:     0,
			Updated:  time.Now(),
		}

		err := mc.Send(ws)

		if err != nil {
			return err
		}

		// Bail out if this is the end
		if !w.run {
			break
		}

		// Wait for the next send
		time.Sleep(time.Duration(5) * time.Second)
	}

	return nil
}
