##!/bin/bash

# Watch the progress:
# cat ./deploy.log | grep elapsed
# or
# less -R ./deploy.log

set -e # Exit on failure
set -x # Print commands

go build ./pkg/cmd/capideploy/capideploy.go

./capideploy list_deployment_resources -prj=./sample.json --verbose > deploy.log

set +x
SECONDS=0
export BILLED_RESOURCES=$(cat deploy.log | grep ",billed")
if [ "$BILLED_RESOURCES" != "" ]; then
  echo "This deployment has resources that may be still/already active, please check the log"
fi

set -x # Print commands

./capideploy create_floating_ips -prj=sample.json --verbose >> deploy.log

set +x

# Save reserved BASTION_IP so we can run capitoolbelt on bastion
export BASTION_IP=$(cat deploy.log | grep "export BASTION_IP=" |  cut -d "=" -f2)
if [ "$BASTION_IP" = "" ]; then
  echo "Cannot retrieve BASTION_IP"
  exit 1
fi

# Configure SSH jumphost so we can run nodetool on Cassandra hosts (requires write access to ~/.ssh/config)
if ! grep -q "$BASTION_IP" ~/.ssh/config; then
  echo "Adding a new jumphost to ~/.ssh/config..."
  echo "" | tee -a ~/.ssh/config
  echo "Host $BASTION_IP" | tee -a ~/.ssh/config
  echo "  User $CAPIDEPLOY_SSH_USER" | tee -a ~/.ssh/config
  echo "  StrictHostKeyChecking=no" | tee -a ~/.ssh/config
  echo "  UserKnownHostsFile=/dev/null" | tee -a ~/.ssh/config
  echo "  IdentityFile  $CAPIDEPLOY_SSH_PRIVATE_KEY_PATH" | tee -a ~/.ssh/config
fi

set -x

./capideploy create_networking -prj=sample.json --verbose >> deploy.log
./capideploy create_security_groups -prj=sample.json --verbose >> deploy.log
./capideploy create_volumes "*" -prj=sample.json --verbose >> deploy.log
./capideploy create_instances "*" -prj=sample.json --verbose >> deploy.log
set +e
until ./capideploy ping_instances '*' -prj=sample.json; do echo "Ping failed, waiting..."; sleep 5; done
set -e

./capideploy attach_volumes "bastion" -prj=sample.json --verbose >> deploy.log
./capideploy install_services "*" -prj=sample.json --verbose >> deploy.log

# Cassandra requires special treatment: stop and start
./capideploy stop_services "cass*" -prj=sample.json --verbose >> deploy.log
sleep 5
./capideploy config_services "cass*" -prj=sample.json --verbose >> deploy.log

./capideploy config_services "bastion,rabbitmq,prometheus,daemon*" -prj=sample.json --verbose >> deploy.log

ssh -o StrictHostKeyChecking=no -i $CAPIDEPLOY_SSH_PRIVATE_KEY_PATH -J $BASTION_IP $CAPIDEPLOY_SSH_USER@10.5.0.11 'nodetool describecluster;nodetool status'

duration=$SECONDS
echo "$(($duration / 60))m $(($duration % 60))s elapsed."

set +x
echo To run commands against this deployment, you will probably need this:
echo export BASTION_IP=$BASTION_IP