// This defines the methods used by the central compilation server.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"sort"
	"sync"
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
	Client string      // Host request a worker
	Addrs  []net.IPNet // IP addresses of the client
}

type WorkerResponse struct {
	Host    string    // Host of the worker
	Address net.IPNet // IP address of the worker
	Port    int       // Port the workers accepts connections on
}

// WorkState represents the load and capacity of a worker
// TODO: replace "Host" with a worker ID struct which contains the host as
// well as the hash of the workers MAC addresses
type WorkerState struct {
	Host     string      // Host the worker resides one
	Addrs    []net.IPNet // IP addresses of the worker
	Port     int         // Port the worker accepts jobs on
	Capacity int         // Number of available cores for building
	Load     int         // How many cores are current in use
	Updated  time.Time   // When the state was last updated
	Speed    float64     // The speed of the worker, computed on the server
}

// List of all currently active works
type WorkerStateList struct {
	Workers []WorkerState
}

// ServerState is all the state of our server
// TODO: consider some kind of channel system instead of a mutex to get
// sync access to these data structures.
type ServerState struct {
	workers map[string]WorkerState // All the currently active workers
	wmutex  *sync.Mutex            // Protects access to workers map

	monitorUpdates *updatePublisher // Sends to multiple channels completion information
}

func NewServerState() *ServerState {
	s := new(ServerState)
	s.workers = make(map[string]WorkerState)
	s.wmutex = new(sync.Mutex)
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

	// Update the shared state
	s.wmutex.Lock()
	defer s.wmutex.Unlock()

	// Keep the current speed if we already have an entry for this host
	if val, ok := s.workers[u.Host]; ok {
		speed := val.Speed
		u.Speed = speed
	}

	s.workers[u.Host] = u
}

// Remove the worker from the current set of workers
func (s *ServerState) removeWorker(h string) {
	s.wmutex.Lock()
	defer s.wmutex.Unlock()

	delete(s.workers, h)
}

// func (s*ServerState) pruneStaleWorkers(h string)

// findWorker finds a free worker which can connect to any of the given
// addresses and return the corresponding address and port
func (s *ServerState) findWorker(addrs []net.IPNet) (string, net.IPNet, int, error) {
	// Error out if we aren't given any addresses to match against
	var empty net.IPNet
	if len(addrs) == 0 {
		return "", empty, 0, errors.New("No source addresses given")
	}

	// Make sure we keep the worker state locked
	s.wmutex.Lock()
	defer s.wmutex.Unlock()

	// Sort the worker IPs so will match local networks before global
	sort.Sort(ByPrivateIPAddr(addrs))

	// Found workers
	var worker WorkerState
	var addr net.IPNet

	worker.Speed = -1
	found := false

	// For now just a simple linear search returning the first free
	for _, wstate := range s.workers {
		space := wstate.Capacity - wstate.Load

		if space > 0 {
			// Get a worker IP address that can connect to the client
			maddr, err := getMatchingIP(addrs, wstate.Addrs)

			// Use this worker if it's faster than the last
			if err == nil && worker.Speed < wstate.Speed {
				worker = wstate
				addr = maddr
				found = true
			}
		}
	}

	// Return the fastest found worker
	if found {
		return worker.Host, addr, worker.Port, nil
	}

	return "", empty, 0, errors.New("No available & reachable host")
}

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
		s.updateStats(m)
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
			s.removeWorker(is.Host)

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
	// Find free worker
	host, addr, port, err := s.findWorker(req.Addrs)

	if err != nil {
		return err
	}

	// Send back result
	r := WorkerResponse{
		Host:    host,
		Address: addr,
		Port:    port,
	}
	return conn.Send(r)
}

// Sends worker state to all monitoring programs
func (s *ServerState) sendWorkState(rate float64) error {
	// Define sleep based our rate
	msSleep := 1 / rate * 1000
	d := time.Duration(int64(msSleep)) * time.Millisecond

	for {
		time.Sleep(d)

		// Copy list into message
		// TODO: maybe reduce copying here?
		var l WorkerStateList

		s.wmutex.Lock()
		for _, state := range s.workers {
			l.Workers = append(l.Workers, state)
		}
		s.wmutex.Unlock()

		// Send out update
		s.monitorUpdates.updates <- l

	}
}

// Updates the workers current speed estimate based on the job results, this
// uses New = Old * 0.9 + Update * 0.1 to try and smooth out spikes caused by
// variability.
func (s *ServerState) updateStats(cj CompletedJob) error {
	s.wmutex.Lock()
	defer s.wmutex.Unlock()

	// Blend in the speed slowly if we already have a speed
	state, ok := s.workers[cj.Worker]

	if !ok {
		return fmt.Errorf("Could not find worker: ", cj.Worker)
	}

	if state.Speed == 0 {
		state.Speed = cj.CompileSpeed
	} else {
		state.Speed = state.Speed*0.9 + cj.CompileSpeed*0.1
	}

	s.workers[cj.Worker] = state

	return nil
}
