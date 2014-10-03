// This defines the methods used by the central compilation server.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"errors"
	"log"
	"net"
	"reflect"
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
	Client string // Host request a worker
}

type WorkerResponse struct {
	Worker string // Host of the worker
	Port   int    // Port the workers accepts connections on
}

// WorkState represents the load and capacity of a worker
type WorkerState struct {
	Host     string    // Host the work resides one
	Port     int       // Port the worker accepts jobs on
	Capacity int       // Number of available cores for building
	Load     int       // How many cores are current in use
	Updated  time.Time // When the state was last updated
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
	// Incoming connections
	for {
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

	s.wmutex.Lock()
	s.workers[u.Host] = u
	s.wmutex.Unlock()
}

// findWorker finds a free worker and returns it's host and port
func (s *ServerState) findWorker() (string, int, error) {
	s.wmutex.Lock()
	defer s.wmutex.Unlock()

	// For now just a simple linear search returning the first free
	for _, state := range s.workers {
		space := state.Capacity - state.Load
		if space > 0 {
			return state.Host, state.Port, nil
		}
	}

	return "", 0, errors.New("No avialable host")
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
		err = s.processWorkerRequest(conn)
	case WorkerState:
		// Push update and then start continously handling the worker connection
		s.updateWorker(m)

		s.handleWorkerConnection(conn)
	case MonitorRequest:
		// Create and register channel which receives information
		u := make(chan interface{})

		s.monitorUpdates.addObs(m.Host, u)

		// Step into our routine which shuffles messages from that channel into
		// the provided connection
		// TODO: a better identifier for this
		s.handleMonitorConnection(conn, m.Host, u)
	case CompletedJob:
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
func (s *ServerState) handleWorkerConnection(conn *MessageConn) {
	for {
		ws, err := conn.ReadWorkerState()

		if err != nil {
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
func (s *ServerState) processWorkerRequest(conn *MessageConn) error {
	// Find free worker
	host, port, err := s.findWorker()

	if err != nil {
		return err
	}

	// Send back result
	r := WorkerResponse{
		Worker: host,
		Port:   port,
	}
	return conn.Send(r)
}
