package main

import (
	"fmt"
	"github.com/jlisee/cbd"
	"os"
	"time"
)

func main() {

	// Make connection to server
	server := os.Getenv("CBD_SERVER")

	for {
		mc, err := connect(server)

		// Keep trying until we get a connection
		if err != nil {
			fmt.Printf("Can't connect to: %s\n", err)
			time.Sleep(time.Duration(1) * time.Second)
			continue
		}

		// Go into an infinite reporting loop
		err = report(mc)

		// A clear exit from report means we should do a graceful shutdown
		if err == nil {
			break
		}
	}
}

// Connects to the the server and requests monitoring data
func connect(address string) (*cbd.MessageConn, error) {
	mc, err := cbd.NewTCPMessageConn(address, time.Duration(10)*time.Second)

	if err != nil {
		return nil, err
	}

	// Get hostname
	hostname, err := os.Hostname()

	if err != nil {
		fmt.Printf("Could not get hostname: %s", err)
		return nil, err
	}

	// Generate a unique host identifier
	hostid := fmt.Sprintf("%s(%d)", hostname, os.Getpid())

	// Send out monitor request
	rq := cbd.MonitorRequest{
		Host: hostid,
	}
	mc.Send(rq)

	return mc, nil
}

// Run forever reporting, return error if something goes wrong
func report(mc *cbd.MessageConn) error {
	for {
		cj, err := mc.ReadCompletedJob()

		if err != nil {
			// TODO: check for stale data
			return err
		}

		// Final output
		// id?  Preprocess/Compile    file.cpp                     server[core#]

		fmt.Printf("%s: finished job in: %.3fs (Speed: %.0f)\n", cj.Worker,
			cj.CompileTime.Seconds(), cj.CompileSpeed)
	}
}
