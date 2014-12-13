#! /bin/bash

# Current directory
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Pull in the build and clean functions
source $DIR/build.sh

# Make sure our working directory is always the script dir
cd $DIR

# Exit on error
set -e

# Clean up initial files, and kill all background jobs
function clean_up() {
    # Remove binaries
    rm -f test-main main.o cbd cbd.test

    # Stop all running processes
    JOBS=$(jobs -p)
    if [ "$JOBS" != "" ]; then
        for jp in $(jobs -p); do
            disown $jp
            kill -9 $jp &> /dev/null
        done
    fi

    # Clean log dir
    if [ -n "$TMPLOGDIR" ]; then
        rm -rf $TMPLOGDIR
        unset TMPLOGDIR
    fi

    # Clear all environment variables
    unset CBD_LOGFILE
    unset CBD_POTENTIAL_HOST
    unset CBD_SERVER
}

function checkout() {
    echo "[Running: ./test-main]"
    testout=$(./test-main)

    if [ "$testout" != "Hello, world!" ]; then
        echo "ERROR: Output Invalid got value '$testout'"
        clean_up
        exit 1
    else
        echo "  GOOD"
    fi
}

function disp() {
    echo
    echo $*
}

clean_up

# Make sure to cleanup on exit
trap clean_up EXIT

# Run tests
disp "[Running tests]"
go test

# Build everything
disp "[Build and install]"
build

# Make sure we don't have any other cbd processes hanging around
cbd_pid=$(pgrep cbd; true)

if [ "$cbd_pid" != "" ]; then
    disp "[Test Check]"
    echo "Error: cbd already running as pid: $cbd_pid"
    exit 1
fi


# ----------------------------------------------------------------------------
# Local tests
# ----------------------------------------------------------------------------

# The compile the program
disp "[Local only test]"

export CBD_POTENTIAL_HOST=''

cbd gcc -c data/main.c -o main.o
cbd gcc main.o -o test-main
checkout # Test the output

# Clean up
clean_up


# ----------------------------------------------------------------------------
# Make sure logging works!
# ----------------------------------------------------------------------------

TMPLOGDIR=`mktemp -d`
export CBD_LOGFILE=$TMPLOGDIR/cbd.log

cbd gcc -c data/main.c -o main.o
cbd gcc main.o -o test-main
checkout # Test the output

LOG_LENGTH=$(wc -l $TMPLOGDIR/cbd.log | cut -d " " -f1)
if [ "$LOG_LENGTH" -lt "7" ]; then
    echo "ERROR: Log too short, contents follow:"
    echo
    cat $TMPLOGDIR/cbd.log
    exit 1
fi

# Clean up
clean_up

# ----------------------------------------------------------------------------
# Server with no server
# ----------------------------------------------------------------------------

disp "[Dead server]"

export CBD_SERVER=":15800"

cbd gcc -c data/main.c -o main.o
cbd gcc main.o -o test-main
checkout # Test the output

clean_up


# ----------------------------------------------------------------------------
# Work with no worker
# ----------------------------------------------------------------------------

disp "[Direct dead worker test]"

export CBD_POTENTIAL_HOST="localhost"

cbd gcc -c data/main.c -o main.o
cbd gcc main.o -o test-main
checkout # Test the output

clean_up


# ----------------------------------------------------------------------------
# Worker test
# ----------------------------------------------------------------------------

# Now lets do it again over the network
disp "[Direct worker test]"

cbd worker &

export CBD_POTENTIAL_HOST="localhost"
export CBD_NO_LOCAL="yes"

cbd gcc -c data/main.c -o main.o
cbd gcc main.o -o test-main
checkout # Test the output

clean_up


# ----------------------------------------------------------------------------
# Server test
# ----------------------------------------------------------------------------

# Now lets do again over with a server and a worker, we also make sure no
# local builds can happen
disp "[Server & worker test]"

export CBD_SERVER="127.0.0.1:15800"
export CBD_NO_LOCAL="yes"

cbd server -port 15800 &

cbd worker -port 15850 &

sleep 1 # Needed hack

cbd gcc -c data/main.c -o main.o
cbd gcc main.o -o test-main
checkout # Test the output

clean_up


# ----------------------------------------------------------------------------
# End of tests
# ----------------------------------------------------------------------------

disp "[Tests Complete]"
