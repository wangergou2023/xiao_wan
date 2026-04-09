#!/bin/bash

go env -w GO111MODULE=on
go env -w GOPROXY=https://goproxy.cn,direct

COMMIT_HASH="$(git rev-parse --short HEAD)"

# optional args: --web-port/-p <port>
WEB_PORT_ARG=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        -p|--web-port)
            if [[ -n "$2" ]]; then
                WEB_PORT_ARG="$2"
                shift 2
            else
                echo "Missing value for $1"
                exit 1
            fi
            ;;
        *)
            shift
            ;;
    esac
done

if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root. sudo ./start.sh"
    exit 1
fi

if [[ -d ./chipper ]]; then
    cd chipper
fi

#if [[ ! -f ./chipper ]]; then
#   if [[ -f ./go.mod ]]; then
#     echo "You need to build chipper first. This can be done with the setup.sh script."
#   else
#     echo "You must be in the chipper directory."
#   fi
#   exit 0
#fi

if [[ ! -f ./source.sh ]]; then
    echo "You need to make a source.sh file. This can be done with the setup.sh script."
    exit 0
fi

source source.sh

# allow override of webserver port
if [[ -n "${WEB_PORT_ARG}" ]]; then
    if [[ "${WEB_PORT_ARG}" =~ ^[0-9]+$ ]]; then
        export WEBSERVER_PORT="${WEB_PORT_ARG}"
    else
        echo "Invalid web port: ${WEB_PORT_ARG}"
        exit 1
    fi
fi

# set go tags
export GOTAGS="nolibopusfile"

if [[ ${USE_INBUILT_BLE} == "true" ]]; then
    GOTAGS="${GOTAGS},inbuiltble"
fi

export GOLDFLAGS="-X 'github.com/kercre123/wire-pod/chipper/pkg/vars.CommitSHA=${COMMIT_HASH}'"

# only whisper
if [[ -f ./chipper ]]; then
    ./chipper
else
    /usr/local/go/bin/go run -tags $GOTAGS -ldflags="${GOLDFLAGS}" cmd/experimental/whisper/main.go
fi
