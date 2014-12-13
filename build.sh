#! /bin/bash

# Exit on error
set -e

# Make sure we are working out of the script dir
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $SCRIPT_DIR

# Clean up initial files
function build() {
    local EXTRA_FLAGS=""
    local OPTIND=1
    local DESTDIR=$GOPATH/bin

    while getopts "s3" o; do
        case "${o}" in
            # Activate static builds
            s)
                export CGO_ENABLED=0
                EXTRA_FLAGS="-a $EXTRA_FLAGS"
                DESTDIR=$GOPATH/bin/linux32
                ;;
            # Cross build for 32 bit linux instead
            3)
                export GOOS=linux
                export GOARCH=386
                ;;
        esac
    done


    local start=$(date +%s.%N)

    go install
    go build $EXTRA_FLAGS cmds/cbd.go

    mkdir -p $DESTDIR
    mv cbd $DESTDIR

    local end=$(date +%s.%N)

    local dur=$(echo "scale=3; ($end - $start)/1" | bc)
    echo "  Duration: ${dur}s"
}


# Bail out here if we are being source
if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
    return
fi

# Parse our command line arguments
OPTIND=1
crosscompile=0

while getopts "h?c" opt; do
    case "$opt" in
    h|\?)
        echo "-c for static linux32 cross compile"
        exit 0
        ;;
    c)  crosscompile=1
        ;;
    esac
done

if [[ $crosscompile == 0 ]]; then
    build
else
    build -s -3
fi
