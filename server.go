// This defines the methods used by the central compilation server.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"log"
	"net"
	"reflect"
	"sort"
	"time"
)

// MonitorRequest is sent from a client that wishes to be sent information
// about the current jobs running on the build cluster.
type MonitorRequest struct {
	Host string
}

// WorkerRequest is sent from the client to the server in order to find
// a worker to process a job
type WorkerRequest struct {
	Client MachineName // Requesting worker
	Addrs  []net.IPNet // IP addresses of the client
}

// Determine what kind of response the server sent
type ResponseType int

const (
	Queued    ResponseType = iota // No data, we are queued
	NoWorkers                     // No workers at all available
	Valid                         // Valid response
)

type WorkerResponse struct {
	Type    ResponseType // Valid or queue
	ID      MachineID    // Uniquely identifies machine
	Host    string       // Host of the worker (for debugging purposes)
	Address net.IPNet    // IP address of the worker
	Port    int          // Port the workers accepts connections on
}

// WorkState represents the load and capacity of a worker
type WorkerState struct {
	ID       MachineID   // Uniquely id for the worker machine
	Host     string      // Host the worker resides one
	Addrs    []net.IPNet // IP addresses of the worker
	Port     int         // Port the worker accepts jobs on
	Capacity int         // Number of available cores for building
	Load     int         // How many cores are current in use
	Updated  time.Time   // When the state was last updated
	Speed    float64     // The speed of the worker, computed on the server
}

// Basic dump of internal state used for monitoring
type ServerStateInfo struct {
	Workers  []WorkerState // List of all currently active workers
	Requests []RequestInfo // Information about all the queued requests
}

// ServerState is all the state of our server
// TODO: consider some kind of channel system instead of a mutex to get
// sync access to these data structures.
type ServerState struct {
	sch Scheduler // Schedules jobs

	monitorUpdates *updatePublisher // Sends to multiple channels completion information
}

func NewServerState() *ServerState {
	s := new(ServerState)
	s.sch = newFifoScheduler()
	s.monitorUpdates = newUpdatePublisher()

	return s
}

// server accepts incoming connections
func (s *ServerState) Serve(ln net.Listener) {
	// Start sending worker updates at 1Hz
	go s.sendWorkState(1)

	// Start up our auto discover server
	var a *discoveryServer
	addr := ln.Addr()
	if taddr, ok := addr.(*net.TCPAddr); ok {
		var err error

		a, err = newDiscoveryServer(taddr.Port)

		if err != nil {
			log.Print("Error starting auto-discovery", err)
			return
		}
		defer a.stop()
	}

	// Incoming connections
	for {
		DebugPrint("Server accepting...")
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}

		// Turn into a message conn
		mc := NewMessageConn(conn, time.Duration(10)*time.Second)

		// Spin off thread to handle the new connection
		go s.handleConnection(mc)
	}
}

// updateWorker updates the worker with the currene state
func (s *ServerState) updateWorker(u WorkerState) {
	// Update time so that we don't have to worry worker clock being
	// in sync with our local clock
	u.Updated = time.Now()

	// Sort the IP addresses, so the most local ones are first, and we can
	// more easily find matching ones in the future
	sort.Sort(ByPrivateIPAddr(u.Addrs))

	// Tell the scheduler about the worker
	s.sch.updateWorker(u)
}

// Remove the worker from the current set of workers
func (s *ServerState) removeWorker(id MachineID) {
	// Have the scheduler remove the worker
	s.sch.removeWorker(id)
}

// func (s*ServerState) pruneStaleWorkers(h string)

// handleMessage decodes incoming messages
func (s *ServerState) handleConnection(conn *MessageConn) {

	// Read the first message on the connection
	_, msg, err := conn.Read()

	if err != nil {
		log.Print("Message reader error: ", err)
		return
	}

	// Hand the message off to the proper function
	switch m := msg.(type) {
	case WorkerRequest:
		err = s.processWorkerRequest(conn, m)
	case WorkerState:
		// Push update and then start continously handling the worker connection
		s.updateWorker(m)

		s.handleWorkerConnection(conn, m)
	case MonitorRequest:
		// Create and register channel which receives information
		u := make(chan interface{})

		s.monitorUpdates.addObs(m.Host, u)

		// Step into our routine which shuffles messages from that channel into
		// the provided connection
		// TODO: a better identifier for this
		s.handleMonitorConnection(conn, m.Host, u)
	case CompletedJob:
		err = s.updateStats(m)

		if err != nil {
			log.Print("Error updating stats: ", err)
		}

		s.monitorUpdates.updates <- m
	default:
		log.Print("Un-handled message type: ", reflect.TypeOf(msg).Name())
	}

	if err != nil {
		log.Printf("Request(%s) error: %s", reflect.TypeOf(msg).Name(),
			err.Error())
	}
}

// handleWorkerConnection continously grabs updates from one worker
// and sends updates the server state
func (s *ServerState) handleWorkerConnection(conn *MessageConn, is WorkerState) {
	for {
		ws, err := conn.ReadWorkerState()

		if err != nil {
			// Drop missing worker
			s.removeWorker(is.ID)

			log.Print("Error reading worker state: ", err)
			break
		}

		s.updateWorker(ws)
	}
}

// handleMonitorConnection sends completed job information to any requested
func (s *ServerState) handleMonitorConnection(conn *MessageConn, h string, cin chan interface{}) {
	for j := range cin {
		err := conn.Send(j)

		// On an error we de-register and bail out
		if err != nil {
			log.Printf("Dropping monitor: %s Error: %s", h, err.Error())
			s.monitorUpdates.removeObs(h)
			break
		}
	}
}

// processWorkerRequest searches for an available worker and sends the
// result back on the given connection.
func (s *ServerState) processWorkerRequest(conn *MessageConn, req WorkerRequest) error {

	// Create a go routine waiting for our scheduling result
	sreq := NewSchedulerRequest(req.Client, req.Addrs)

	errOut := make(chan error)

	go func() {
		var err error

		// Keep telling the waiting client we are queued
	Loop:
		for {
			// 1 second timeout
			timeout := make(chan bool, 1)
			go func() {
				time.Sleep(1 * time.Second)
				timeout <- true
			}()

			// Wait for timeouts, or requests
			select {
			case result := <-sreq.r:
				// We got a result!, send it to the user
				err = conn.Send(result)

				// If it's valid break out of our loop
				if result.Type == Valid {
					break Loop
				}

			case <-timeout:
				// the read from ch has timed, tell the user we have a queue
				// result
				err = conn.Send(WorkerResponse{Type: Queued})

				// Cancel the request and leave the loop
				if err != nil {
					cerr := s.sch.cancel(sreq.guid)

					if cerr != nil {
						DebugPrintf("Error canceling request %s: %s", sreq, cerr)
					}

					break Loop
				}
			}
		}

		errOut <- err
	}()

	// Schedule our request
	s.sch.schedule(sreq)

	// Wait for the scheduler to respond, and the message to send
	return <-errOut
}

// Sends worker state to all monitoring programs
func (s *ServerState) sendWorkState(rate float64) error {
	// Define sleep based our rate
	msSleep := 1 / rate * 1000
	d := time.Duration(int64(msSleep)) * time.Millisecond

	for {
		// Copy list into message
		// TODO: maybe reduce copying here?
		si := s.sch.getStateInfo()

		// Send out update
		s.monitorUpdates.updates <- si

		time.Sleep(d)
	}
}

// Updates scheduler state based on completed job information
func (s *ServerState) updateStats(cj CompletedJob) error {
	return s.sch.completed(cj)
}
