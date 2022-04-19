#!/bin/sh

systemctl daemon-reload
systemctl enable zackup
systemctl restart zackup
