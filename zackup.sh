#!/bin/sh

export ZACKUP_ROOT="$(pwd)/testdata"
exec go run main.go $*
