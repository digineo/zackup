#!/bin/sh

make zackup.linux
exec ./zackup.linux --root "$(pwd)/testdata" $*
