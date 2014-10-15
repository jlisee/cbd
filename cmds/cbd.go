package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/jlisee/cbd"
)

func main() {

	// Input flag parsing
	var port uint
	var server bool

	flag.UintVar(&port, "port", cbd.DefaultPort, "Port to listen on")
	flag.BoolVar(&server, "server", false, "Run as a server instead of worker")

	flag.Parse()

	// Do work
	log.Print("Listening on: ", port, " server?: ", server)

	if server {
		runServer(int(port))
	} else {
		runWorker(int(port))
	}
}

func runWorker(port int) {
	log.Print("Worker starting")

	// Listen on any address
	address := "0.0.0.0:" + strconv.FormatUint(uint64(port), 10)

	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	saddr := os.Getenv("CBD_SERVER")

	w, err := cbd.NewWorker(port, saddr)
	if err != nil {
		log.Fatal(err)
	}

	w.Serve(ln)
}

func runServer(port int) {
	log.Print("Server starting")

	// Listen on any address
	address := ":" + strconv.FormatUint(uint64(port), 10)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	s := cbd.NewServerState()

	s.Serve(ln)
}
