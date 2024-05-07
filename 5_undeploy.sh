##!/bin/bash

set -e # Exit on failure
set -x # Print commands

./capideploy stop_services "*" -prj=sample.json --verbose >> undeploy.log
sleep 5
./capideploy detach_volumes "*" -prj=sample.json --verbose >> undeploy.log
./capideploy delete_volumes "*" -prj=sample.json --verbose >> undeploy.log
./capideploy delete_instances "*" -prj=sample.json --verbose > undeploy.log

./capideploy delete_security_groups -prj=sample.json --verbose >> undeploy.log
./capideploy delete_networking -prj=sample.json --verbose >> undeploy.log
./capideploy delete_floating_ips -prj=sample.json --verbose >> undeploy.log

./capideploy list_deployment_resources -prj=./sample.json
