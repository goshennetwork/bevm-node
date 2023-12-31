#!/usr/bin/env bash
set -ex

VERSION=$(git describe --always --tags --long)
echo $VERSION

if [ $RUNNER_OS == 'Linux' ]; then
  echo "linux sys"
  env
  export GOPATH="/home/runner/go"
  #go test -v ./...
  export GOPRIVATE=github.com/ontology-layer-2

  go mod tidy

  bash ./.gha.gofmt.sh

  make geth

  #quit when meet first fail test
  for s in $(go list ./...); do if ! go test -failfast -v -p 1 $s; then break; fi; done
  fi
