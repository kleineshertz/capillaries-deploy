if [ "$PROMETHEUS_NODE_EXPORTER_VERSION" = "" ]; then
  echo Error, missing: PROMETHEUS_NODE_EXPORTER_VERSION=1.8.2
 exit 1
fi

# It may complain if already exists, do not react to stderr
sudo useradd --no-create-home --shell /bin/false node_exporter 2>/dev/null

if [ "$(uname -p)" == "x86_64" ]; then
ARCH=amd64
else
ARCH=arm64
fi

# Download node exporter
EXPORTER_DL_FILE=node_exporter-$PROMETHEUS_NODE_EXPORTER_VERSION.linux-$ARCH
cd ~
sudo rm -f $EXPORTER_DL_FILE.tar.gz
echo Downloading https://github.com/prometheus/node_exporter/releases/download/v$PROMETHEUS_NODE_EXPORTER_VERSION/$EXPORTER_DL_FILE.tar.gz ...
curl -LOs https://github.com/prometheus/node_exporter/releases/download/v$PROMETHEUS_NODE_EXPORTER_VERSION/$EXPORTER_DL_FILE.tar.gz
if [ "$?" -ne "0" ]; then
    echo Cannot download, exiting
    exit $?
fi
tar xvf $EXPORTER_DL_FILE.tar.gz

sudo cp $EXPORTER_DL_FILE/node_exporter /usr/local/bin
sudo chown node_exporter:node_exporter /usr/local/bin/node_exporter

rm -rf $EXPORTER_DL_FILE.tar.gz $EXPORTER_DL_FILE

