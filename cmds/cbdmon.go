package main

import (
	"fmt"
	"github.com/jlisee/cbd"
	"log"
	"os"
	"time"
)

func main() {
	// Make connection to server
	server := os.Getenv("CBD_SERVER")

	mc, err := cbd.NewTCPMessageConn(server, time.Duration(10)*time.Second)

	if err != nil {
		return
	}

	// Get hostname
	hostname, err := os.Hostname()

	if err != nil {
		return
	}

	// Generate a unique host identifier
	hostid := fmt.Sprintf("%s(%d)", hostname, os.Getpid())

	// Send out monitor request
	rq := cbd.MonitorRequest{
		Host: hostid,
	}
	mc.Send(rq)

	// Read back results
	for {
		cj, err := mc.ReadCompletedJob()

		if err != nil {
			log.Print("Error reading job response: ", err)
			break
		}

		log.Printf("Worker: %s completed job for: %s", cj.Client, cj.Worker)
	}
}
