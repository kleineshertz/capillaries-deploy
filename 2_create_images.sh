##!/bin/bash

set -e # Exit on failure
set -x # Print commands

SECONDS=0
./capideploy stop_services "*" -p sample.jsonnnet -v
./capideploy detach_volumes "bastion" -p sample.jsonnnet -v

# We want to be 100% sure that cassandra has stopped
sleep 10

./capideploy create_snapshot_images "*" -p sample.jsonnnet -v
./capideploy delete_instances "*" -p sample.jsonnnet -v
./capideploy list_deployment_resources -p sample.jsonnet -v
duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."
