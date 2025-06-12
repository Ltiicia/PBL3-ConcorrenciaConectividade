#!/bin/bash

chmod +x empresa2.sh

docker-compose build
docker-compose create
docker ps -a
docker-compose start empresa_002 broker
docker-compose logs -f empresa_002

# cd ./exec
# ./empresa2.sh
