##!/bin/bash

set -e # Exit on failure
set -x # Print commands

SECONDS=0
./capideploy stop_services "*" -prj=sample.json --verbose
sleep 10
./capideploy detach_volumes "bastion" -prj=sample.json --verbose

# We want to be 100% sure that cassandra has stopped
sleep 10

./capideploy create_snapshot_images "*" -prj=sample.json --verbose
./capideploy delete_instances "*" -prj=sample.json --verbose
./capideploy list_deployment_resources -prj=./sample.json --verbose
duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."
