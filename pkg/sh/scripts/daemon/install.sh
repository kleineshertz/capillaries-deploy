# Add all used Python modules here
# No need to install venv or pip, just proceed with python3-xyz
sudo NEEDRESTART_MODE=a apt-get install -y python3-dateutil

CAPI_BINARY=capidaemon

if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

if [ "$CAPILLARIES_RELEASE_URL" = "" ]; then
  echo Error, missing: CAPIDEPLOY_CAPILLARIES_RELEASE_URL=https://capillaries-release.s3.us-east-1.amazonaws.com/latest
  exit 1
fi

if [ "$OS_ARCH" = "" ]; then
  echo Error, missing: $CAPIDEPLOY_OS_ARCH=linux/amd64
  exit 1
fi

if [ ! -d /home/$SSH_USER/bin ]; then
  mkdir -p /home/$SSH_USER/bin
fi

cd /home/$SSH_USER/bin
curl -LO $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.gz
if [ "$?" -ne "0" ]; then
    echo "Cannot download $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.gz to /home/$SSH_USER/bin"
    exit $?
fi
curl -LO $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.json
if [ "$?" -ne "0" ]; then
    echo "Cannot download from $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.json to /home/$SSH_USER/bin"
    exit $?
fi
gzip -d -f $CAPI_BINARY.gz
chmod 744 $CAPI_BINARY