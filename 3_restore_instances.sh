##!/bin/bash
set -e # Exit on failure
set -x # Print commands

SECONDS=0

./capideploy create_instances_from_snapshot_images "*" -p sample.jsonnnet -v

./capideploy ping_instances '*' -p sample.jsonnet -n 20

./capideploy attach_volumes "bastion" -p sample.jsonnnet -v
./capideploy start_services "*" -p sample.jsonnnet -v

# Cassandra requires one more cycle to embrace the fact that data/log firectories /data0,/data1 are gone
./capideploy stop_services "cass*" -p sample.jsonnnet -v
./capideploy start_services "cass*" -p sample.jsonnnet -v

duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."

./capideploy list_deployment_resources -p sample.jsonnet -v
