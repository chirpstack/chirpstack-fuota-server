#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    create role chirpstack_fuota with login password 'chirpstack_fuota';
    create database chirpstack_fuota with owner chirpstack_fuota;
EOSQL
