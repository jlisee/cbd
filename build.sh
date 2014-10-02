#! /bin/bash

# Exit on error
set -e

# Clean up initial files
function build() {
    local start=$(date +%s.%N)

    go install
    go build cmds/cbdcc.go
    go build cmds/cbdmon.go
    go build cmds/cbd.go
    mv cbdcc cbdmon cbd $GOPATH/bin

    local end=$(date +%s.%N)

    local dur=$(echo "scale=3; ($end - $start)/1" | bc)
    echo "  Duration: ${dur}s"
}


# Bail out here if we are being source
if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
    return
fi

build
