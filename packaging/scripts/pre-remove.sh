#!/usr/bin/env bash

NAME=chirpstack-fuota-server

# remove systemd
systemctl stop $NAME
systemctl disable $NAME
