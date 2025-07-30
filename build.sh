#!/bin/bash


# Run chmod +x build.sh to make it executable.
# Example usage:
# Windows 64-bit: ./build.sh -system windows -bits 64
# Linux 32-bit: ./build.sh -system linux -bits 32
# Raspberry Pi 32-bit: ./build.sh -system raspberry -bits 32
# macOS 64-bit: ./build.sh -system macos -bits 64
# This script builds the offline-net application for different systems and architectures.
# Executable files will be placed in the current directory.

set -e

APP_NAME="offline-net"
BUILD_DIR="."
GOFILE="main.go"

# Default values
SYSTEM=""
BITS=""

usage() {
  echo "Usage: $0 -system <windows|linux|macos|raspberry> -bits <32|64>"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -system)
      SYSTEM="$2"
      shift 2
      ;;
    -bits)
      BITS="$2"
      shift 2
      ;;
    *)
      usage
      ;;
  esac
done

if [[ -z "$SYSTEM" || -z "$BITS" ]]; then
  usage
fi

mkdir -p "$BUILD_DIR"

case "$SYSTEM" in
  windows)
    GOOS="windows"
    EXT=".exe"
    if [[ "$BITS" == "64" ]]; then
      GOARCH="amd64"
    elif [[ "$BITS" == "32" ]]; then
      GOARCH="386"
    else
      usage
    fi
    ;;
  linux)
    GOOS="linux"
    EXT="-linux"
    if [[ "$BITS" == "64" ]]; then
      GOARCH="amd64"
    elif [[ "$BITS" == "32" ]]; then
      GOARCH="386"
    else
      usage
    fi
    ;;
  macos)
    GOOS="darwin"
    EXT="-macos"
    if [[ "$BITS" == "64" ]]; then
      GOARCH="amd64"
    elif [[ "$BITS" == "32" ]]; then
      echo "32-bit macOS builds are not supported."
      exit 1
    else
      usage
    fi
    ;;
  raspberry)
    GOOS="linux"
    EXT="-raspberry"
    GOARCH="arm"
    if [[ "$BITS" == "32" ]]; then
      GOARM="7"
    elif [[ "$BITS" == "64" ]]; then
      GOARCH="arm64"
      unset GOARM
    else
      usage
    fi
    ;;
  *)
    usage
    ;;
esac

OUT="$BUILD_DIR/$APP_NAME$EXT"

echo "Building for $SYSTEM $BITS-bit..."

if [[ "$SYSTEM" == "raspberry" && "$BITS" == "32" ]]; then
  GOOS=$GOOS GOARCH=$GOARCH GOARM=$GOARM go build -o "$OUT" "$GOFILE"
else
  GOOS=$GOOS GOARCH=$GOARCH go build -o "$OUT" "$GOFILE"
fi

echo "Build complete: $OUT"