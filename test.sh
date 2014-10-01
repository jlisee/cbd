#! /bin/bash

# Current directory
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Pull in the build and clean functions
source $DIR/build.sh

# Exit on error
set -e

# Clean up initial files
function clean() {
    rm -f test-main main.o cbd cbd.test
}

function checkout() {
    echo "[Running: ./test-main]"
    testout=$(./test-main)

    if [ "$testout" != "Hello, world!" ]; then
        echo "Output Invalid got value '$testout'"
        exit 1
    else
        echo "  GOOD"
    fi
}

function disp() {
    echo
    echo $*
}

clean

# Run tests
disp "[Running tests]"
go test

# Build everything
disp "[Build and install]"
build

# ----------------------------------------------------------------------------
# Local tests
# ----------------------------------------------------------------------------

# The compile the program
disp "[Local only test]"

export CBD_POTENTIAL_HOST=''

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main
checkout # Test the output

# Clean up
clean


# ----------------------------------------------------------------------------
# Server with no server
# ----------------------------------------------------------------------------

disp "[Dead server]"

unset CBD_POTENTIAL_HOST
export CBD_SERVER=":15800"

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main
checkout # Test the output

clean


# ----------------------------------------------------------------------------
# Work with no worker
# ----------------------------------------------------------------------------

disp "[Direct dead worker test]"

export CBD_POTENTIAL_HOST="localhost"
unset CBD_SERVER

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main
checkout # Test the output

clean


# ----------------------------------------------------------------------------
# Worker test
# ----------------------------------------------------------------------------

# Now lets do it again over the network
disp "[Direct worker test]"

cbd &
d_pid=$!
trap "kill -9 ${d_pid}" EXIT

export CBD_POTENTIAL_HOST="localhost"
unset CBD_SERVER

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main
checkout # Test the output

clean
kill -9 ${d_pid} &> /dev/null


# ----------------------------------------------------------------------------
# Server test
# ----------------------------------------------------------------------------

# Now lets do again over with a server and a worker
disp "[Server & worker test]"

unset CBD_POTENTIAL_HOST
export CBD_SERVER="localhost:15800"

cbd -address $CBD_SERVER -server &
a_pid=$!
trap "kill -9 ${a_pid}" EXIT

cbd -address ":15786" &
d_pid=$!
trap "kill -9 ${d_pid}" EXIT

sleep 1 # Needed hack

cbdcc gcc -c data/main.c -o main.o
cbdcc gcc main.o -o test-main
checkout # Test the output

clean
