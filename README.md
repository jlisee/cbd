cbuildd
=======

A distributed build and caching engine for C and C++.

Plan
=====

 - Build a program which can compile splits the compile into a preprocess and
   then build step.

Functional Parts
=================

 - Initial command line processor:
   - Determines if this a command to produce a .o file
   - Identifies output target

 - Command line transformers:
   - Produces preprocessor generation command
   - Produces final compilation command

 - Thing that executes gcc commands

Design
=======