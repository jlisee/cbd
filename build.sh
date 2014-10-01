#! /bin/bash

# Exit on error
set -e

# Clean up initial files
function build() {
    go install
    go build cmds/cbdcc.go
    go build cmds/cbdmon.go
    go build cmds/cbd.go
    mv cbdcc cbdmon cbd $GOPATH/bin
}


# Bail out here if we are being source
if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
    return
fi

build
