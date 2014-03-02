#! /bin/bash

# Exit on error
set -e

# Clean up initial files
function clean() {
    rm -f test-main main.o cbd cbd.test
}

function checkout() {
    testout=$(./test-main)

    if [ "$testout" != "Hello, world!" ]; then
        echo "Output Invalid got value '$testout'"
        exit 1
    fi
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
checkout # Test the output


# Clean up
clean

# Now lets do it again over the network
cbd &
d_pid=$!
trap "kill -9 ${d_pid}" EXIT

export CBD_POTENTIAL_HOST="localhost"

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main
checkout # Test the output

clean

# Now lets do again over with a server
