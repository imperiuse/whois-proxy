#!/usr/bin/env bash

# Останавливаем
./dev-down.sh

# Все команды выполняются на VM!
if [[ $1 == '' ]]
then

    echo "Run with deployer"
    # 1. Build локального докер образа сервиса
    docker build -f .docker/Dockerfile --target=app --build-arg CONFIG="dev_config.yml" -t local/whois-proxy .

elif [[ $1 == 'debug' ]]
then

    echo "Run service with docker-compose"
    # 1. Build локального докер образа сервиса
    docker build -f .docker/Dockerfile --target=app-debug --build-arg CONFIG="dev_config.yml" -t local/whois-proxy .

    # 2. Запуск docker-compose сервиса
    docker-compose -f docker-compose.override.yml up -d

    # 3. Запуск сервиса в отдельной консоли
    docker exec -it whois-proxy_app_1 ./whois-proxy server

elif [[ $1 == 'debug_dlv' ]]
then

    echo "Run dlv debugger with docker-compose "
    # 1. Build локального докер образа сервиса
    docker build -f .docker/Dockerfile --target=app-debug --build-arg CONFIG="dev_config.yml" -t local/whois-proxy .

    # 2. Запуск docker-compose сервиса
    docker-compose -f docker-compose.override.yml up -d

    # 3. Запуск сервиса в отдельной консоли в режиме отладки (dlv)
    docker exec -it whois-proxy_app_1 ./dlv --headless --listen=:2345 --api-version=2 --accept-multiclient exec ./whois-proxy

elif [[ $1 == 'test' ]]
then

    go test -timeout 5m -race $(go list ./... | grep -v /vendor/)
    ./tool/coverage.sh

else
    echo "Incorrect parameter"
fi
