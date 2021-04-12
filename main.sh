#!/usr/bin/env bash

#
# Prepare the local shell context with any globally needed env vars
#
environment() {
  export SERVICE_NAME=prty
  export ENV_REF=${1:-local}
  export SERVICE_PORT=3200

  export GIT_VER=$(git rev-parse --short HEAD)
  export COMPOSE_NETWORK=${SERVICE_NAME}_${GIT_VER}
  export DOCKER_TAG="inburst/${SERVICE_NAME}:${GIT_VER}"
}

#
# Docker Helpers
#
cleanup_compose_network() {
  docker network rm ${COMPOSE_NETWORK} || true > /dev/null 2>&1 # ignore missing network
}
cleanup_compose_containers() {
  docker-compose down -t 1 --remove-orphans > /dev/null 2>&1
  docker volume prune -f
}
cleanup_compose() {
  cleanup_compose_containers
  cleanup_compose_network
}
make_docker_network() {
  cleanup_compose_network
  docker network create -d bridge ${COMPOSE_NETWORK}
}



# BUILD_TARGET can be local, testable, and deployable
cmd_build() {
  environment
  BUILD_TARGET="${1:-deployable}"

  echo "Building container for ${BUILD_TARGET} ${SERVICE_NAME}"

  docker build \
      --build-arg SERVICE_NAME=${SERVICE_NAME} \
      --target ${BUILD_TARGET} \
      -t ${DOCKER_TAG} .
}

# local development
cmd_local() {
  environment

  if [[ $1 = "rebuild" ]]; then
    REBUILD_FLAGS="--build --force-recreate"
    echo "build with flags: ${REBUILD_FLAGS}"
  fi

  start() {
    # clean up
    cmd_build local
    docker run -it ${DOCKER_TAG}
    

    #docker-compose down -v > /dev/null 2>&1
    #export BUILD_TARGET="local"

    #make_docker_network
    #docker-compose up ${REBUILD_FLAGS} --remove-orphans -d

    # immediately bind to service logs
    #cmd_logs ${SERVICE_NAME}
  }

  stop() {
    echo "Stopping infrastructure within docker-compose"
    cleanup_compose
  }

  if [[ $1 = "stop" ]]; then
    stop
    exit 0
  fi

  start
}

# runs tests
# add -w or watch to the command for watch mode
cmd_tests() {
  environment

  # run tests
  export BUILD_TARGET="local"
  #docker-compose exec ${SERVICE_NAME} bash -c "bash ./entrypoints/test.sh ${1}"
  docker-compose exec ${SERVICE_NAME} bash -c "./entrypoints/test.sh ${1}"
}


# tail logs up to 3 given container names
cmd_logs() {
  environment
  docker-compose logs -f ${1} ${2} ${3}
}

# Get a shell into a specific container
cmd_sh() {
  environment
  docker-compose exec ${1} /bin/bash
}


################################################

#
# Print all available commands
#
help() {
  echo "Usage:
  $ ./main.sh COMMAND

  Where COMMAND can be:
      local               # Start local development
      local stop          # Stop local development
      test [-w]           # Run tests. Add -w for watch mode

      build               # Build Docker container

      logs CONTAINER_NAME # Get logs from CONTAINER_NAME
      sh CONTAINER_NAME   # Run shell in CONTAINER_NAME
  " 1>&2
  exit 1
}

#
# Command translation from arg to func call
# If you wish to accesas a new function you must add a hook here
#
case "$1" in
  # local development
  local)
    cmd_local ${@:2};;
  test)
    cmd_tests ${@:2};;
  # docker build
  build)
    cmd_build ${@:2};;
  # helper tools
  logs)
    cmd_logs ${@:2};;
  sh)
    cmd_sh ${@:2};;
  # fall back
  *)
    help; exit 1
esac
