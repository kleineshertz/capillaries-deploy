##!/bin/bash
set -e # Exit on failure
set -x # Print commands

SECONDS=0

./capideploy create_instances_from_snapshot_images "*" -p sample.jsonnet -v
#./capideploy create_instances_from_snapshot_images "bastion" -p sample.jsonnet -v

./capideploy ping_instances '*' -p sample.jsonnet -n 50
#./capideploy ping_instances 'bastion' -p sample.jsonnet -n 50

./capideploy attach_volumes "bastion" -p sample.jsonnet -v
./capideploy start_services "*" -p sample.jsonnet -v
#./capideploy start_services "bastion" -p sample.jsonnet -v

# Cassandra requires one more cycle to embrace the fact that data/log firectories /data0,/data1 are gone
./capideploy stop_services "cass*" -p sample.jsonnet -v
./capideploy start_services "cass*" -p sample.jsonnet -v

duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."

./capideploy list_deployment_resources -p sample.jsonnet -v
