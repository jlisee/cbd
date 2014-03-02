package main

import (
	"flag"
	"github.com/jlisee/cbd"
	"log"
	"net"
	"os"
	"strconv"
)

func main() {

	// Input flag parsing
	var address string
	daddress := "localhost:" + strconv.Itoa(cbd.Port)

	var server bool

	flag.StringVar(&address, "address", daddress, "Address to listen on")
	flag.BoolVar(&server, "server", false, "Run as a server instead of worker")

	flag.Parse()

	// Do work
	log.Print("Listening on: ", address, " server?: ", server)

	if server {
		runServer(address)
	} else {
		runWorker(address)
	}
}

func runWorker(address string) {
	log.Print("Worker starting")

	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	saddr := os.Getenv("CBD_SERVER")

	w, err := cbd.NewWorker(address, saddr)
	if err != nil {
		log.Fatal(err)
	}

	w.Serve(ln)
}

func runServer(address string) {
	log.Print("Server starting")

	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	s := cbd.NewServerState()

	s.Serve(ln)
}
