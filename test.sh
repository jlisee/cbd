#! /bin/bash

# Exit on error
set -e

# Clean up initial files
rm -f test-main main.o cbuildd

# Build everything
go build

# The compile the program
./cbuildd -c data/main.c -o main.o
./cbuildd main.o -o test-main

# Maybe we should test the output somehow
./test-main

# Clean up
rm -f test-main main.o