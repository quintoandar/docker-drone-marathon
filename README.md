# drone-marathon

Drone plugin for deploying applications to a [Marathon](https://mesosphere.github.io/marathon/) server.

## Docker

Build the Docker image with the following commands:

```
docker build --rm=true -t quintoandar/drone-marathon .
```

## Usage

Execute from the working directory:

```
docker run --rm \
  -e PLUGIN_SERVER=http://marathon.mesos:8080 \
  -e PLUGIN_MARATHONFILE=marathon.yaml \
  -v $(pwd):$(pwd) \
  -w $(pwd) \
  quintoandar/drone-marathon
```

