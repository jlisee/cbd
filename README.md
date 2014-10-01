cbd
====

A distributed build and caching engine for C and C++.


Usage
======

On your "build-server" host start the server listening on port 18000:

    cbd -address :18000 -server

Start up workers on your various hosts and point them toward the
server (they will be listening on port 17000):

    export CBD_SERVER=build-server:18000
    cbd -address :17000

Use the client program in place of gcc and g++:

    export CBD_SERVER=build-server:18000
    export CC='cbdcc gcc'
    export CXX='cbdcc g++'

Now run your build tool as normal.


Roadmap
========

 - Monitoring CLI program
 - Auto-discovery of server
 - Centralized object file cache
 - Embedded web browser for monitoring GUI


Development
============

The shell script "test.sh" runs the unit tests as well as some integration
tests. The standard "go test" will run the unit tests. The "build.sh" program
will build the software and install the binaries on the GOPATH.


Design
=======

The main parts of the final system and their purpose:

Server
-------

 - Tracks the load status of workers.
 - Responds to requests sending back an available worker

Worker
-------

 - Accepts build jobs, compiles and returns the results
 - Sends updates containing load and CPU count to the server

Client
-------

 - Stands in for the compiler turning commands into build jobs
 - Uses the server to find a worker to complete it's job
