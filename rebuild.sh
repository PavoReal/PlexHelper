#!/bin/bash
set -e

CGO_ENABLED=0 go build -o plex-helper .
sudo docker compose down
sudo docker compose up -d --build
