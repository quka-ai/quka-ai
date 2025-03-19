#!/bin/sh 

cd `dirname $0`
cd ../

SERVICE="quka"
SUB_SERVICE="service"
BUILD_DIR=${PWD}/_build

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o ${BUILD_DIR}/${SERVICE} ./cmd/

mkdir -p ${BUILD_DIR}/etc

cp -r ./cmd/${SUB_SERVICE}/etc/ ${BUILD_DIR}/etc/