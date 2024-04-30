CAPI_BINARY=capitoolbelt

if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

if [ "$CAPILLARIES_RELEASE_URL" = "" ]; then
  echo Error, missing: CAPILLARIES_RELEASE_URL=https://capillaries-release.s3.us-east-1.amazonaws.com/latest
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
curl -LOs $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.gz
if [ "$?" -ne "0" ]; then
    echo "Cannot download $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.gz to /home/$SSH_USER/bin"
    exit $?
fi
curl -LOs $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.json
if [ "$?" -ne "0" ]; then
    echo "Cannot download from $CAPILLARIES_RELEASE_URL/$OS_ARCH/$CAPI_BINARY.json to /home/$SSH_USER/bin"
    exit $?
fi
gzip -d -f $CAPI_BINARY.gz
chmod 744 $CAPI_BINARY
