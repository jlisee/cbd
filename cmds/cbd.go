package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/jlisee/cbd"
)

func main() {
	// Pull of the first argument and either use it as a command, or the
	// compiler otherwise (later make this a just a configuration setting)
	command := os.Args[1]

	// Input flag parsing
	port := new(uint)

	flag.UintVar(port, "port", cbd.DefaultPort, "Port to listen on")
	//flag.BoolVar(&server, "server", false, "Run as a server instead of worker")

	// Command map
	commands := map[string]func(){
		"server": func() {
			runServer(int(*port))
		},
		"worker": func() {
			runWorker(int(*port))
		},
		"monitor": func() {
			runMonitor()
		},
	}

	// Attempt to lookup the command, if it's not in the map, we treat it as the
	// compiler
	fn, ok := commands[command]

	if ok {
		// Shift the args down one, to ignore the first command, if we don't
		// do this the flag package basically ignores all arguments
		os.Args = os.Args[1:]
		flag.Parse()

		// Now run our command
		fn()
	} else {
		// We have to parse the arguments manually because the default flag
		// package stops parsing after positional args, and
		// github.com/ogier/pflag errors out on unknown arguments.
		runCompiler(command, os.Args[2:])
	}
}

func runWorker(port int) {
	log.Print("Worker starting, port: ", port)

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
	log.Print("Server starting, port: ", port)

	// Listen on any address
	address := ":" + strconv.FormatUint(uint64(port), 10)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

	s := cbd.NewServerState()

	s.Serve(ln)
}

func runMonitor() {
	log.Print("Monitor starting")

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

func runCompiler(compiler string, args []string) {
	// Parse compiler arguments
	b := cbd.ParseArgs(args)

	// Setup logging if needed
	logpath := os.Getenv("CBD_LOGFILE")

	if len(logpath) > 0 {
		// Open the log file for appending
		f, err := os.OpenFile(logpath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)

		if err != nil {
			log.Fatal(err)
		}

		defer f.Close()

		log.SetOutput(f)
		log.Print("ARGS: ", os.Args[2:])
		log.Printf("  Distribute?: %t", b.Distributable)
		log.Printf("  Output path:  %s[%d]\n", b.Output(), b.Oindex)
		log.Printf("  Input path:   %s[%d]\n", b.Input(), b.Iindex)

		cbd.DebugLogging = true
	}
	// Dump arguments

	// TODO: Add in a local compile fast past
	if b.Distributable {
		// Pre-process the file into a compile job
		job, results, err := cbd.MakeCompileJob(compiler, b)

		if err != nil {
			fmt.Print(string(results.Output))
			cbd.DebugPrint("Preprocess Error: ", string(results.Output))
			os.Exit(results.Return)
		}

		// See if we have a remote host defined
		cresults, err := cbd.ClientBuildJob(job)

		if err != nil || cresults.Return != 0 {
			fmt.Print(string(cresults.Output))
			cbd.DebugPrint("Build Error: ", string(cresults.Output))
			os.Exit(cresults.Return)
		}

		// Now write the results to right output location
		f, err := os.Create(b.Output())

		if err != nil {
			log.Fatal(err)
		}

		defer f.Close()

		_, err = f.Write(cresults.ObjectCode)

		if err != nil {
			log.Fatal(err)
		}

		cbd.DebugPrint("Remote Success")

	} else {
		results, err := cbd.RunCmd(compiler, os.Args[2:])

		if err != nil {
			fmt.Print(string(results.Output))
			cbd.DebugPrint("Local Error: ", string(results.Output))
			os.Exit(results.Return)
		}

		cbd.DebugPrint("Success")
	}

}
