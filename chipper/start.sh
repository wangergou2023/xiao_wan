#!/bin/bash

UNAME=$(uname -a)
COMMIT_HASH="$(git rev-parse --short HEAD)"

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

# set go tags
export GOTAGS="nolibopusfile"

if [[ ${USE_INBUILT_BLE} == "true" ]]; then
    GOTAGS="${GOTAGS},inbuiltble"
fi

export GOLDFLAGS="-X 'github.com/wangergou2023/xiao_wan/chipper/pkg/vars.CommitSHA=${COMMIT_HASH}'"

#./chipper
if [[ -f ./chipper ]]; then
    ./chipper
else
    /usr/local/go/bin/go run -tags $GOTAGS -ldflags="${GOLDFLAGS}" cmd/experimental/bigmodel/main.go
fi
