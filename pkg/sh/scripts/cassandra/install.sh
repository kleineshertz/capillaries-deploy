if [ "$JMX_EXPORTER_VERSION" = "" ]; then
  echo Error, missing: JMX_EXPORTER_VERSION=0.20.0
  exit 1
fi
if [ "$NVME_REGEX" = "" ]; then
  echo Error, missing: NVME_REGEX="nvme[0-9]n[0-9] 558.8G"
  exit 1
fi

echo "deb https://debian.cassandra.apache.org 41x main" | sudo tee -a /etc/apt/sources.list.d/cassandra.sources.list
curl https://downloads.apache.org/cassandra/KEYS | sudo apt-key add -

sudo apt-get -y update

#iostat
sudo apt-get install -y sysstat

# Cassandra requires Java 8
sudo apt-get install -y openjdk-8-jdk openjdk-8-jre

sudo apt-get install -y cassandra

sudo systemctl status cassandra
if [ "$?" -ne "0" ]; then
    echo Bad cassandra service status, exiting
    exit $?
fi

# JMX Exporter
curl -LO https://repo1.maven.org/maven2/io/prometheus/jmx/jmx_prometheus_javaagent/$JMX_EXPORTER_VERSION/jmx_prometheus_javaagent-$JMX_EXPORTER_VERSION.jar
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
mount_device(){
	mount_dir="/data"$1
	device_name=$2
	sudo mkfs -t xfs /dev/$device_name
	sudo mkdir $mount_dir
	sudo mount /dev/$device_name $mount_dir
	sudo chown cassandra $mount_dir
	sudo chmod 777 $mount_dir;
}

# "nvme[0-9]n[0-9] 558.8G"
# "loop[0-9] [0-9.]+M"
device_number=0
lsblk | awk '{print $1,$4}' | grep -E "$NVME_REGEX" | awk '{print $1}' |
while read -r device_name; do
  mount_device $device_number $device_name
  device_number=$((device_number+1)) 
done

echo $device_number disks matching: $NVME_REGEX
