#!/bin/sh

case "$1" in
    remove)
        systemctl daemon-reload
    ;;
esac
