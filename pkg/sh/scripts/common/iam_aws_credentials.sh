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
sudo mkdir '~/.aws'
sudo cat > ~/.aws/credentials << 'endmsgmarker'
[default]
aws_access_key_id=$IAM_AWS_ACCESS_KEY_ID
aws_secret_access_key=$IAM_AWS_SECRET_ACCESS_KEY
endmsgmarker
sudo cat > ~/.aws/config << 'endmsgmarker'
[default]
region=$IAM_AWS_DEFAULT_REGION
output=json
endmsgmarker
