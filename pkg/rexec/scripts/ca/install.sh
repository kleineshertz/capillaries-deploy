if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

if [ "$CAPILLARIES_RELEASE_URL" = "" ]; then
  echo Error, missing: CAPILLARIES_RELEASE_URL=https://capillaries-release.s3.us-east-1.amazonaws.com/latest
 exit 1
fi

rm -fR /home/$SSH_USER/ca
mkdir -p /home/$SSH_USER/ca
cd /home/$SSH_USER/ca
curl -LOs $CAPILLARIES_RELEASE_URL/ca/ca.tgz
if [ "$?" -ne "0" ]; then
    echo "Cannot download ca from $CAPILLARIES_RELEASE_URL/ca/ca.tgz to /home/$SSH_USER/ca"
    exit $?
fi

tar xvzf ca.tgz
sudo chmod 644 *
rm ca.tgz
