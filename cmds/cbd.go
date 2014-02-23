package main

import (
	"net"
	"github.com/jlisee/cbuildd"
	"log"
	"encoding/gob"
	"strconv"
)


func main() {

	address := ":" + strconv.Itoa(cbuildd.Port)
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
	dec := gob.NewDecoder(conn)
	var job cbuildd.CompileJob

	// TODO: use SetReadDeadline to timeout if we get nothing back
	err := dec.Decode(&job)

	if err != nil {
		log.Print("Decode error:", err)
		return
	}

	// Build the code
	cresults, _ := job.Compile()

	// Send back the result
	enc := gob.NewEncoder(conn)

	err = enc.Encode(cresults)

	if err != nil {
		log.Print("Encode error:", err)
		return
	}

	log.Print("Done.")
}
