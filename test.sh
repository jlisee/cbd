#! /bin/bash

# Exit on error
set -e

# Clean up initial files
function clean() {
    rm -f test-main main.o cbd cbd.test
}

clean

# Run tests
go test

# Build everything
go install
go build cmds/cbdcc.go
go build cmds/cbd.go
mv cbdcc cbd $GOPATH/bin

# The compile the program
export CBD_POTENTIAL_HOST=''

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main

# Maybe we should test the output somehow
./test-main

# Clean up
clean

# Now lets do it again over the network
cbd &
d_pid=$!
trap "kill -9 ${d_pid}" EXIT

export CBD_POTENTIAL_HOST="localhost"

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main
./test-main

clean
