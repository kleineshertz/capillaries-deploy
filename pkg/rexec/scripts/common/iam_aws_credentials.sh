if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi
if [ "$IAM_AWS_ACCESS_KEY_ID" = "" ]; then
  echo Error, missing: IAM_AWS_ACCESS_KEY_ID=AK...
  exit 1
fi
if [ "$IAM_AWS_SECRET_ACCESS_KEY" = "" ]; then
  echo Error, missing: IAM_AWS_SECRET_ACCESS_KEY=...
  exit 1
fi
if [ "$IAM_AWS_DEFAULT_REGION" = "" ]; then
  echo Error, missing: IAM_AWS_DEFAULT_REGION=us-east-1
  exit 1
fi

# Credentials and config for S3 access
rm -fR /home/$SSH_USER/.aws
mkdir -p /home/$SSH_USER/.aws

sudo echo "[default]" > /home/$SSH_USER/.aws/credentials
sudo echo "aws_access_key_id=$IAM_AWS_ACCESS_KEY_ID" >> /home/$SSH_USER/.aws/credentials
sudo echo "aws_secret_access_key=$IAM_AWS_SECRET_ACCESS_KEY" >> /home/$SSH_USER/.aws/credentials

sudo echo "[default]" > /home/$SSH_USER/.aws/config
sudo echo "region=$IAM_AWS_DEFAULT_REGION" >> /home/$SSH_USER/.aws/config
sudo echo "output=json" >> /home/$SSH_USER/.aws/config
