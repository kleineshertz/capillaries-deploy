##!/bin/bash

set +e # Continue on failure
set -x # Print commands

SECONDS=0
./capideploy stop_services "*" -p sample.jsonnet -v

set -e # Exit on failure
./capideploy detach_volumes "bastion" -p sample.jsonnet -v

# We want to be 100% sure that cassandra has stopped
#sleep 10

./capideploy create_snapshot_images "*" -p sample.jsonnet -v
#./capideploy create_snapshot_images "bastion" -p sample.jsonnet -v
./capideploy delete_instances "*" -p sample.jsonnet -v
#./capideploy delete_instances "bastion" -p sample.jsonnet -v
./capideploy list_deployment_resources -p sample.jsonnet -v
duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."
