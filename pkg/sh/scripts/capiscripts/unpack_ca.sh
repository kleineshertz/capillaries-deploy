if [ "$CAPI_BINARY_ROOT" = "" ]; then
  echo Error, missing: CAPI_BINARY_ROOT=/home/$SSH_USER/bin
  exit 1
fi

echo "Unpacking ca..."
cd $CAPI_BINARY_ROOT/ca
tar xvzf all.tgz
chmod 644 *
rm all.tgz