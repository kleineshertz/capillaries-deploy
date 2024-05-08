##!/bin/bash
set -e # Exit on failure
set -x # Print commands

SECONDS=0

./capideploy create_instances_from_snapshot_images "*" -prj=sample.json --verbose

set +x
until ./capideploy ping_instances '*' -prj=sample.json; do echo "Ping failed, waiting..."; sleep 5; done
set -x

./capideploy attach_volumes "bastion" -prj=sample.json --verbose
./capideploy config_services "*" -prj=sample.json --verbose

# Cassandra requires one more cycle to embrace the fact that data/log firectories /data0,/data1 are gone
./capideploy stop_services "cass*" -prj=sample.json --verbose
sleep 5
./capideploy start_services "cass*" -prj=sample.json --verbose

duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."

./capideploy list_deployment_resources -prj=./sample.json --verbose
