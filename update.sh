#!/usr/bin/env bash
set -e

[ ! -d "/opt/agentgrid" ] && git clone https://github.com/francescobianco/agentgrid /opt/agentgrid

cd /opt/agentgrid || exit 1

git pull --no-rebase

#chmod 777 data/

cp -f compose.override.example compose.override.yml

docker compose up -d --build --force-recreate
