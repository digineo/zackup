#!/bin/sh

case "$1" in
    remove)
        systemctl disable zackup || true
        systemctl stop zackup || true
    ;;
esac
