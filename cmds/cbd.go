package main

import (
	"github.com/jlisee/cbd"
	"log"
	"net"
	"strconv"
	"time"
)

func main() {

	address := ":" + strconv.Itoa(cbd.Port)
	log.Print("Listening on: ", address)

	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	log.Print("Handling request...")

	// Decode the CompileJob
	mc := cbd.NewMessageConn(conn, time.Duration(10)*time.Second)
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
