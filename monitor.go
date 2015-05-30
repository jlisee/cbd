// Functions and structures relating to the monitoring the state of the cluster
// This is a basic observer pattern implementation.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"fmt"
	"os"
	"reflect"
	"time"
)

// Convenient wrapper for ID & Host
type MachineName struct {
	ID   MachineID // The unique ID of the machine
	Host string    // The host name of the machine
}

func (mn *MachineName) ToString() string {
	return fmt.Sprintf("%s[%s]", mn.Host, mn.ID)
}

// CompletedJob is one updated about a job completed on the cluster
type CompletedJob struct {
	Client       MachineName   // Machine that requested the job
	Worker       MachineName   // Worker that build the job
	InputSize    int           // Bytes of source code compiled
	OutputSize   int           // Bytes of object code produced
	CompileTime  time.Duration // How long the job took to complete
	CompileSpeed float64       // Speed rating used for the job
}

// We define the compile speed of a job based
func (c *CompletedJob) computeCompileSpeed() {
	c.CompileSpeed = float64(c.OutputSize) / c.CompileTime.Seconds() / 1024
}

type Monitor struct {
	saddr string       // Address for the server
	mc    *MessageConn // Message connection to server
}

// Creates a new connects to the server
func NewMonitor(saddr string) *Monitor {
	m := new(Monitor)
	m.saddr = saddr
	m.mc = nil

	return m
}

// Connect to the server and sends monitoring request
func (m *Monitor) Connect() error {
	// If we have no set address, use auto-discovery to find the server
	saddr := m.saddr

	if len(saddr) == 0 {
		DebugPrint("Finding server with autodiscovery")
		var err error
		saddr, err = audoDiscoverySearch(time.Duration(5) * time.Second)

		if err != nil {
			return err
		}
	} else {
		saddr = addPortIfNeeded(saddr, DefaultServerPort)
	}

	// Connect
	var err error
	m.mc, err = NewTCPMessageConn(saddr, time.Duration(10)*time.Second)

	if err != nil {
		m.mc = nil
		return err
	}

	// Get hostname
	hostname, err := os.Hostname()

	if err != nil {
		fmt.Printf("Could not get hostname: %s", err)
		m.mc = nil
		return err
	}

	// Generate a unique host identifier
	hostid := fmt.Sprintf("%s(%d)", hostname, os.Getpid())

	// Send out monitor request
	rq := MonitorRequest{
		Host: hostid,
	}
	m.mc.Send(rq)

	return nil
}

// Print out report data in raw form, connecting if needed
func (m *Monitor) BasicReport() error {
	// Connect of needed
	if m.mc == nil {
		err := m.Connect()

		if err != nil {
			return err
		}
	}

	// Jump into our reporting loop
	for {
		_, i, err := m.mc.Read()

		if err != nil {
			// TODO: check for stale data
			m.mc = nil

			return err
		}

		switch m := i.(type) {
		case CompletedJob:
			fmt.Printf("%s: finished job in: %.3fs (Speed: %.0f)\n", m.Worker,
				m.CompileTime.Seconds(), m.CompileSpeed)

		case WorkerStateList:
			// Final output
			// id?  Preprocess/Compile    file.cpp                     server[core#]

			fmt.Printf("[")
			for _, state := range m.Workers {
				// element is the element from someSlice for where we are
				fmt.Printf("%s[%d|%d|%.0f] ", state.Host, state.Load,
					state.Capacity, state.Speed)
			}
			fmt.Printf("]\n")

		default:
			fmt.Printf("ERROR, unknown message type: %s\n",
				reflect.TypeOf(i).Name())
		}
	}
}
