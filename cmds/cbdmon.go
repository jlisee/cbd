package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jlisee/cbd"
)

func main() {

	// Make connection to server
	server := os.Getenv("CBD_SERVER")

	m := cbd.NewMonitor(server)
	for {
		err := m.Connect()

		// Keep trying until we get a connection
		if err != nil {
			fmt.Printf("Can't connect to: %s\n", err)
			time.Sleep(time.Duration(1) * time.Second)
			continue
		}

		// Go into an infinite reporting loop
		err = m.BasicReport()

		// A clear exit from report means we should do a graceful shutdown
		if err == nil {
			break
		}
	}
}
