#!/bin/bash

# ensure all required packages are installed
if [[ -f /usr/bin/apt ]]; then
    TARGET="debian"
    echo "Debian-based Linux detected."
elif [[ -f /usr/bin/pacman ]]; then
    TARGET="arch"
    echo "Arch Linux detected."
elif [[ -f /usr/bin/dnf ]]; then
    TARGET="fedora"
    echo "Fedora/openSUSE detected."
fi

if [[ ${TARGET} == "debian" ]]; then
    sudo apt update -y
    sudo apt install -y wget openssl net-tools libsox-dev libopus-dev make iproute2 xz-utils libopusfile-dev pkg-config gcc curl g++ unzip avahi-daemon git libasound2-dev libsodium-dev
elif [[ ${TARGET} == "arch" ]]; then
    sudo pacman -Sy --noconfirm
    sudo pacman -S --noconfirm wget openssl net-tools sox opus make iproute2 opusfile curl unzip avahi git libsodium
elif [[ ${TARGET} == "fedora" ]]; then
    sudo dnf update
    sudo dnf install -y wget openssl net-tools sox opus make opusfile curl unzip avahi git libsodium-devel
fi

if [[ ! -d ./chipper ]]; then
  echo "This must be run in the wire-pod/ directory."
  exit 1
fi

git fetch --all
git reset --hard origin/main
if [[ -f ./chipper/chipper ]]; then
    cd chipper
    source source.sh
    sudo systemctl stop wire-pod
    echo "wire-pod.service created, building chipper with Whisper STT service..."
    sudo /usr/local/go/bin/go build cmd/experimental/whisper/main.go
    echo "Syncing..."
    sync
    sudo systemctl daemon-reload
    sudo systemctl start wire-pod
    echo "wire-pod is now running with the updated code!"
fi
echo
echo "Updated successfully!"
echo
