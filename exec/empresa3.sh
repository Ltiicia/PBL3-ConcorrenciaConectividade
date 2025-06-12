#!/bin/bash

chmod +x empresa3.sh

docker-compose build
docker-compose create
docker ps -a
docker-compose start empresa_003 broker
docker-compose logs -f empresa_003

# cd ./exec
# ./empresa3.sh
