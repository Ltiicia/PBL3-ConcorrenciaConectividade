#!/bin/bash

chmod +x inicia.sh

docker-compose build
docker-compose create
docker ps -a
docker-compose start empresa_001 broker
docker-compose logs -f empresa_001

# Para o veiculo:
# docker-compose start veiculo
# docker exec -it veiculo sh
# ./veiculo

# cd ./exec
# ./inicia.sh
