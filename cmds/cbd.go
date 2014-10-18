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

// Hold information about each command
type Command struct {
	fn    func()   // Function to execute
	help  string   // Help text to display to the user
	flags []string // The list of flags this command takes
	alias bool     // When true it's an alias for another
}

// Return true if the command uses this flag
func (c Command) hasFlag(flag string) bool {
	for _, v := range c.flags {
		if v == flag {
			return true
		}
	}
	return false
}

// Print the program usage header
func printUsage() {
	fmt.Printf(`cbd is a distributed C/C++ build tool.

Usage:
	cbd command [arguments]
`)
}

// Print the help information for each command
func printCommandHelp(cmds map[string]Command) {
	// Basic usage
	printUsage()

	// Find the longest command for formatting purposes
	l := 0

	for name, cmd := range cmds {
		if !cmd.alias {
			nl := len(name)
			if nl > l {
				l = nl
			}
		}
	}

	// Form our format string
	f := "  %" + strconv.Itoa(l) + "s - %s\n"

	// Print our commands
	fmt.Printf("\nThe commands are:\n\n")
	for name, cmd := range cmds {
		if !cmd.alias {
			fmt.Printf(f, name, cmd.help)
		}
	}
}

func main() {
	// Input arguments
	port := new(uint)
	server := new(string)

	// Command map
	commands := make(map[string]Command)
	cmdsPointer := &commands

	cmdUpdate := map[string]Command{
		"server": {
			fn: func() {
				runServer(int(*port))
			},
			help:  "Run central scheduler",
			flags: []string{"port"},
		},
		"worker": {
			fn: func() {
				runWorker(int(*port), *server)
			},
			help:  "Run build slave",
			flags: []string{"port", "server"},
		},
		"monitor": {
			fn: func() {
				runMonitor(*server)
			},
			help:  "Run monitoring CLI",
			flags: []string{"server"},
		},
		"help": {
			fn: func() {
				printCommandHelp(*cmdsPointer)
			},
			help: "Display help information",
		},
	}

	// Add in our helpful command aliases
	helpAliases := []string{"-help", "-h", "--help"}

	aliasCmd := cmdUpdate["help"]
	aliasCmd.alias = true

	for _, name := range helpAliases {
		cmdUpdate[name] = aliasCmd
	}

	// Update our main command list
	for name, cmd := range cmdUpdate {
		commands[name] = cmd
	}

	// Pull of the first argument as our command.  Fall back on help if needed,
	// otherwise this is the compiler the user wants to run.  (later make this
	// a just a configuration setting)
	command := "help"

	// Grab our command then Shift the args down one, to ignore the first
	// command, if we don't do this the flag package basically ignores all
	// arguments
	if len(os.Args) > 1 {
		command = os.Args[1]

		if len(os.Args) > 1 {
			os.Args = os.Args[1:]
		}
	}

	// Attempt to lookup the command, if it's not in the map, we treat it as the
	// compiler
	cmd, ok := commands[command]

	if ok {
		if cmd.hasFlag("port") {
			flag.UintVar(port, "port", cbd.DefaultPort, "Port to listen on")
		}
		if cmd.hasFlag("server") {
			defS := os.Getenv("CBD_SERVER")
			flag.StringVar(server, "server", defS, "Address of the server")
		}

		flag.Parse()

		// Now run our command
		cmd.fn()
	} else {
		// We have to parse the arguments manually because the default flag
		// package stops parsing after positional args, and
		// github.com/ogier/pflag errors out on unknown arguments.
		runCompiler(command, os.Args[1:])
	}
}

func runWorker(port int, saddr string) {
	log.Print("Worker starting, port: ", port)

	// Listen on any address
	address := "0.0.0.0:" + strconv.FormatUint(uint64(port), 10)

	ln, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}

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

func runMonitor(server string) {
	log.Print("Monitor starting")

	// Make connection to server
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

		cbd.DebugLogging = true
	}

	// Dump arguments
	cbd.DebugPrint("ARGS: ", args)
	cbd.DebugPrintf("  Distribute?: %t", b.Distributable)
	cbd.DebugPrintf("  Output path:  %s[%d]\n", b.Output(), b.Oindex)
	cbd.DebugPrintf("  Input path:   %s[%d]\n", b.Input(), b.Iindex)

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
		results, err := cbd.RunCmd(compiler, args)

		if err != nil {
			fmt.Print(string(results.Output))
			cbd.DebugPrint("Local Error: ", string(results.Output))
			os.Exit(results.Return)
		}

		cbd.DebugPrint("Success")
	}

}
