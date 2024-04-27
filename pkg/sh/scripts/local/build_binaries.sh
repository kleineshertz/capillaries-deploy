#!/bin/bash
if [ "$DIR_CAPILLARIES_ROOT" = "" ]; then
  echo Error, missing DIR_CAPILLARIES_ROOT=~/src/capillaries
  exit 1
fi
if [ "$DIR_BUILD_LINUX_AMD64" = "" ]; then
  echo Error, missing DIR_BUILD_LINUX_AMD64=~/src/capillaries/build/linux/amd64
  exit 1
fi
if [ "$DIR_BUILD_LINUX_ARM64" = "" ]; then
  echo Error, missing DIR_BUILD_LINUX_ARM64=~/src/capillaries/build/linux/arm64
  exit 1
fi
if [ "$DIR_PKG_EXE" = "" ]; then
  echo Error, missing DIR_PKG_EXE=~/src/capillaries/pkg/exe
  exit 1
fi
if [ "$DIR_CODE_PARQUET" = "" ]; then
  echo Error, missing DIR_CODE_PARQUET=~/src/capillaries/test/code/parquet
  exit 1
fi
if [ "$DIR_SRC_CA" = "" ]; then
  echo Error, missing DIR_CA=~/src/capillaries/test/ca
  exit 1
fi
if [ "$DIR_BUILD_CA" = "" ]; then
  echo Error, missing DIR_CA=~/src/capillaries/build/ca
  exit 1
fi

cd "$DIR_CAPILLARIES_ROOT"
echo "Building Capillaries binaries from " \"$(pwd)\"
echo "$DIR_BUILD_LINUX_AMD64"
echo "$DIR_BUILD_LINUX_ARM64"
echo "$DIR_PKG_EXE"
echo "$DIR_CODE_PARQUET"
echo "$DIR_SRC_CA" "$DIR_BUILD_CA"

# Assuming HOME is set by ExecLocal
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export GOCACHE="$HOME/.cache/go-build"
export GOMODCACHE="$HOME/go/pkg/mod"

if [ ! -d "$DIR_BUILD_LINUX_AMD64" ]; then
  mkdir -p "$DIR_BUILD_LINUX_AMD64"
fi

if [ ! -d "$DIR_BUILD_LINUX_ARM64" ]; then
  mkdir -p "$DIR_BUILD_LINUX_ARM64"
fi

if [ ! -d "$DIR_BUILD_CA" ]; then
  mkdir -p "$DIR_BUILD_CA"
fi

GOOS=linux GOARCH=amd64 go build -o "$DIR_BUILD_LINUX_AMD64/capidaemon" -ldflags="-s -w" "$DIR_PKG_EXE/daemon/capidaemon.go"
gzip -f "$DIR_BUILD_LINUX_AMD64/capidaemon"
GOOS=linux GOARCH=amd64 go build -o "$DIR_BUILD_LINUX_AMD64/capiwebapi" -ldflags="-s -w" "$DIR_PKG_EXE/webapi/capiwebapi.go"
gzip -f "$DIR_BUILD_LINUX_AMD64/capiwebapi"
GOOS=linux GOARCH=amd64 go build -o "$DIR_BUILD_LINUX_AMD64/capitoolbelt" -ldflags="-s -w" "$DIR_PKG_EXE/toolbelt/capitoolbelt.go"
gzip -f "$DIR_BUILD_LINUX_AMD64/capitoolbelt"
GOOS=linux GOARCH=amd64 go build -o "$DIR_BUILD_LINUX_AMD64/capiparquet" -ldflags="-s -w" "$DIR_CODE_PARQUET/capiparquet.go"
gzip -f "$DIR_BUILD_LINUX_AMD64/capiparquet"

GOOS=linux GOARCH=arm64 go build -o "$DIR_BUILD_LINUX_ARM64/capidaemon" -ldflags="-s -w" "$DIR_PKG_EXE/daemon/capidaemon.go"
gzip -f "$DIR_BUILD_LINUX_ARM64/capidaemon"
GOOS=linux GOARCH=arm64 go build -o "$DIR_BUILD_LINUX_ARM64/capiwebapi" -ldflags="-s -w" "$DIR_PKG_EXE/webapi/capiwebapi.go"
gzip -f "$DIR_BUILD_LINUX_ARM64/capiwebapi"
GOOS=linux GOARCH=arm64 go build -o "$DIR_BUILD_LINUX_ARM64/capitoolbelt" -ldflags="-s -w" "$DIR_PKG_EXE/toolbelt/capitoolbelt.go"
gzip -f "$DIR_BUILD_LINUX_ARM64/capitoolbelt"
GOOS=linux GOARCH=arm64 go build -o "$DIR_BUILD_LINUX_ARM64/capiparquet" -ldflags="-s -w" "$DIR_CODE_PARQUET/capiparquet.go"
gzip -f "$DIR_BUILD_LINUX_ARM64/capiparquet"

pushd "$DIR_SRC_CA"
tar cvzf "$DIR_BUILD_CA/all.tgz" *
popd
