if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

if [ "$S3_AWS_DEFAULT_REGION" = "" ]; then
  echo Error, missing: S3_AWS_DEFAULT_REGION=us-east-1
  exit 1
fi

# Credentials and config for S3 access only
rm -fR /home/$SSH_USER/.aws
mkdir -p /home/$SSH_USER/.aws

sudo echo "[default]" > /home/$SSH_USER/.aws/config
sudo echo "region=$S3_AWS_DEFAULT_REGION" >> /home/$SSH_USER/.aws/config
sudo echo "output=json" >> /home/$SSH_USER/.aws/config
