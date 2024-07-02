if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

if [ "$CAPILLARIES_RELEASE_URL" = "" ]; then
  echo Error, missing: CAPILLARIES_RELEASE_URL=https://capillaries-release.s3.us-east-1.amazonaws.com/latest
 exit 1
fi

rm -fR /home/$SSH_USER/ui
mkdir -p /home/$SSH_USER/ui
cd /home/$SSH_USER/ui
curl -LOs $CAPILLARIES_RELEASE_URL/webui/webui.tgz
if [ "$?" -ne "0" ]; then
    echo "Cannot download webui from $CAPILLARIES_RELEASE_URL/webui/webui.tgz to /home/$SSH_USER/ui"
    exit $?
fi

tar xvzf webui.tgz
rm webui.tgz
