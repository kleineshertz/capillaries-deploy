##!/bin/bash

set -e # Exit on failure
set -x # Print commands

./capideploy stop_services "*" -prj=sample.json --verbose
sleep 5
./capideploy detach_volumes "bastion" -prj=sample.json --verbose

# We want to be 100% sure that cassandra has stopped
sleep 10

./capideploy create_snapshot_images "*" -prj=sample.json --verbose
./capideploy delete_instances "*" -prj=sample.json --verbose
./capideploy list_deployment_resources -prj=./sample.json --verbose
