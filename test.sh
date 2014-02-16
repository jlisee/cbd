#! /bin/bash

# Exit on error
set -e

# Clean up initial files
rm -f test-main main.o cbuildd

# Build everything
go install
go build cmds/cbdcc.go
mv cbdcc $GOPATH/bin

# The compile the program
cbdcc -c data/main.c -o main.o
cbdcc main.o -o test-main

# Maybe we should test the output somehow
./test-main

# Clean up
rm -f test-main main.o
