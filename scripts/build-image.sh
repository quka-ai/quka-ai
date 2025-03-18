#!/bin/bash
IMAGE_PROJECT=$1
IMAGE_NAME=brew-api
VERSION=$2

# Make full image name
IMAGE=${IMAGE_PROJECT}/${IMAGE_NAME}:${VERSION}

docker build -t ${IMAGE} --platform linux/amd64 . --network=host --build-arg HTTP_PROXY=$HTTP_PROXY --build-arg HTTPS_PROXY=$HTTPS_PROXY

docker push ${IMAGE_PROJECT}/${IMAGE_NAME}:${VERSION}
