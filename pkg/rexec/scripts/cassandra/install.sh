if [ "$CASSANDRA_VERSION" = "" ]; then
  echo Error, missing: CASSANDRA_VERSION=50x
  exit 1
fi

if [ "$JMX_EXPORTER_VERSION" = "" ]; then
  echo Error, missing: JMX_EXPORTER_VERSION=1.0.1
  exit 1
fi

echo "deb https://debian.cassandra.apache.org $CASSANDRA_VERSION main" | sudo tee -a /etc/apt/sources.list.d/cassandra.sources.list
# apt-key is deprecated. but still working, just silence it
curl -s https://downloads.apache.org/cassandra/KEYS | sudo apt-key add - 2>/dev/null

# To avoid "Key is stored in legacy trusted.gpg keyring" in stderr
cd /etc/apt
sudo cp trusted.gpg trusted.gpg.d
cd ~

# apt-get -y install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code

sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

#iostat
# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y sysstat 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo sysstat install error, exiting
    exit $?
fi

# Cassandra requires Java 8
# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y openjdk-8-jdk openjdk-8-jre 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo openjdk install error, exiting
    exit $?
fi

# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y cassandra 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo cassandra install error, exiting
    exit $?
fi

sudo systemctl status cassandra
if [ "$?" -ne "0" ]; then
    echo Bad cassandra service status, exiting
    exit $?
fi

# JMX Exporter
curl -LOs https://repo1.maven.org/maven2/io/prometheus/jmx/jmx_prometheus_javaagent/$JMX_EXPORTER_VERSION/jmx_prometheus_javaagent-$JMX_EXPORTER_VERSION.jar
if [ "$?" -ne "0" ]; then
    echo Cannot download JMX exporter, exiting
    exit $?
fi
sudo mv jmx_prometheus_javaagent-$JMX_EXPORTER_VERSION.jar /usr/share/cassandra/lib/
sudo chown cassandra /usr/share/cassandra/lib/jmx_prometheus_javaagent-$JMX_EXPORTER_VERSION.jar

# JMX Exporter config
cat > jmx_exporter.yml << 'endmsgmarker'
lowercaseOutputLabelNames: true
lowercaseOutputName: true
whitelistObjectNames: ["org.apache.cassandra.metrics:*"]
# ColumnFamily is an alias for Table metrics
blacklistObjectNames: ["org.apache.cassandra.metrics:type=ColumnFamily,*"]
rules:
# Generic gauges with 0-2 labels
- pattern: org.apache.cassandra.metrics<type=(\S*)(?:, ((?!scope)\S*)=(\S*))?(?:, scope=(\S*))?, name=(\S*)><>Value
  name: cassandra_$1_$5
  type: GAUGE
  labels:
    "$1": "$4"
    "$2": "$3"

#
# Emulate Prometheus 'Summary' metrics for the exported 'Histogram's.
# TotalLatency is the sum of all latencies since server start
#
- pattern: org.apache.cassandra.metrics<type=(\S*)(?:, ((?!scope)\S*)=(\S*))?(?:, scope=(\S*))?, name=(.+)?(?:Total)(Latency)><>Count
  name: cassandra_$1_$5$6_seconds_sum
  type: UNTYPED
  labels:
    "$1": "$4"
    "$2": "$3"
  # Convert microseconds to seconds
  valueFactor: 0.000001

- pattern: org.apache.cassandra.metrics<type=(\S*)(?:, ((?!scope)\S*)=(\S*))?(?:, scope=(\S*))?, name=((?:.+)?(?:Latency))><>Count
  name: cassandra_$1_$5_seconds_count
  type: UNTYPED
  labels:
    "$1": "$4"
    "$2": "$3"

- pattern: org.apache.cassandra.metrics<type=(\S*)(?:, ((?!scope)\S*)=(\S*))?(?:, scope=(\S*))?, name=(.+)><>Count
  name: cassandra_$1_$5_count
  type: UNTYPED
  labels:
    "$1": "$4"
    "$2": "$3"

- pattern: org.apache.cassandra.metrics<type=(\S*)(?:, ((?!scope)\S*)=(\S*))?(?:, scope=(\S*))?, name=((?:.+)?(?:Latency))><>(\d+)thPercentile
  name: cassandra_$1_$5_seconds
  type: GAUGE
  labels:
    "$1": "$4"
    "$2": "$3"
    quantile: "0.$6"
  # Convert microseconds to seconds
  valueFactor: 0.000001

- pattern: org.apache.cassandra.metrics<type=(\S*)(?:, ((?!scope)\S*)=(\S*))?(?:, scope=(\S*))?, name=(.+)><>(\d+)thPercentile
  name: cassandra_$1_$5
  type: GAUGE
  labels:
    "$1": "$4"
    "$2": "$3"
    quantile: "0.$6"
endmsgmarker
sudo mv jmx_exporter.yml /etc/cassandra/
sudo chown cassandra /etc/cassandra/jmx_exporter.yml

# Let Cassandra know about JMX Exporter and config
echo 'JVM_OPTS="$JVM_OPTS -javaagent:/usr/share/cassandra/lib/jmx_prometheus_javaagent-'$JMX_EXPORTER_VERSION'.jar=7070:/etc/cassandra/jmx_exporter.yml"' | sudo tee -a /etc/cassandra/cassandra-env.sh

# For now stop it. We will reconfigure it anywways
echo Stopping Cassandra after installation...
sudo systemctl stop cassandra

# Cassandra 50 has a habit to make it drwxr-x---. Make it drwxr-xr-x
echo Changing /var/log/cassandra permissions...
sudo chmod 755 /var/log/cassandra

# RAM disk size in GB
# export RAM_DISK_SIZE=$(awk '/MemFree/ { printf "%.0f\n", $2/1024/2 }' /proc/meminfo)
# echo $RAM_DISK_SIZE
# sudo mkdir /mnt/ramdisk
# sudo chmod 777 /mnt/ramdisk
# sudo mount -t tmpfs -o size="$RAM_DISK_SIZE"m myramdisk /mnt/ramdisk
# if [ "$?" -ne "0" ]; then
#     echo Cannot mount ramdisk, exiting
#     exit $?
# fi

# It requires cassandra user, so do not run it before you install Cassandra
# This will mount NVME devices to /data0, /data1, etc
# mount_device(){
# 	mount_dir="/data"$1
# 	device_name=$2
# 	sudo mkfs -t xfs /dev/$device_name
# 	sudo mkdir $mount_dir
# 	sudo mount /dev/$device_name $mount_dir
# 	sudo chown cassandra $mount_dir
# 	sudo chmod 777 $mount_dir;
# }

# # "nvme[0-9]n[0-9] 558.8G"
# # "loop[0-9] [0-9.]+M"
# device_number=0
# lsblk | awk '{print $1,$4}' | grep -E "$NVME_REGEX" | awk '{print $1}' |
# while read -r device_name; do
#   mount_device $device_number $device_name
#   device_number=$((device_number+1)) 
# done

# echo $device_number disks matching: $NVME_REGEX
