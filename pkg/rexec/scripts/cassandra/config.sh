# https://www.jamescoyle.net/how-to/2448-create-a-simple-cassandra-cluster-with-3-nodes
# https://www.digitalocean.com/community/tutorials/how-to-install-cassandra-and-run-a-single-node-cluster-on-ubuntu-22-04
# https://youkudbhelper.wordpress.com/2020/05/17/cassandradaemon-java731-cannot-start-node-if-snitchs-data-center-dc1-differs-from-previous-data-center-datacenter1/
# https://stackoverflow.com/questions/38961502/cannot-start-cassandra-snitchs-datacenter-differs-from-previous

if [ "$CASSANDRA_SEEDS" = "" ]; then
  echo Error, missing: export CASSANDRA_SEEDS=10.5.0.11,10.5.0.12,10.5.0.13,10.5.0.14
  exit 1
fi
if [ "$CASSANDRA_IP" = "" ]; then
  echo Error, missing: export CASSANDRA_IP=10.5.0.11
  exit 1
fi

if [ "$NVME_REGEX" = "" ]; then
  echo Error, missing: export NVME_REGEX="nvme[0-9]n[0-9] 558.8G"
  exit 1
fi

if [[ "$NVME_REGEX" != nvme* ]]; then
  echo Error, NVME_REGEX has unexpected format $NVME_REGEX
  exit 1
fi

if [ "$(sudo systemctl status cassandra | grep running)" != "" ]; then
  >&2 echo Cassandra is running, stop it before configuring it
  exit 1
fi

sudo sed -i -e "s~seeds:[\: \"a-zA-Z0-9\.,]*~seeds: $CASSANDRA_SEEDS~g" /etc/cassandra/cassandra.yaml
sudo sed -i -e "s~listen_address:[\: \"a-zA-Z0-9\.]*~listen_address: $CASSANDRA_IP~g" /etc/cassandra/cassandra.yaml
sudo sed -i -e "s~rpc_address:[\: \"a-zA-Z0-9\.]*~rpc_address: $CASSANDRA_IP~g" /etc/cassandra/cassandra.yaml
sudo sed -i -e "s~endpoint_snitch:[\: \"a-zA-Z0-9\.]*~endpoint_snitch: SimpleSnitch~g" /etc/cassandra/cassandra.yaml
#sudo sed -i -e "s~prepared_statements_cache_size:[ a-zA-Z0-9]*~prepared_statements_cache_size: 500MiB~g" /etc/cassandra/cassandra.yaml

# Data on attached volume. Comment out to store data on the ephemeral instance volume at /var/lib/cassandra/data.
#sudo sed -i -e "s~- /var/lib/cassandra/data~- /data/d~g" /etc/cassandra/cassandra.yaml
#sudo sed -i -e "s~- /var/lib/cassandra/data~- /mnt/ramdisk/data~g" /etc/cassandra/cassandra.yaml
sudo sed -i -e "s~- /var/lib/cassandra/data~~g" /etc/cassandra/cassandra.yaml
# One disk or two disks (Cassandra instances can have one ore two nvme drives)
if [ -d "/data1" ]; then
  sudo sed -i -e "s~data_file_directories:[^\n]*~data_file_directories: [ /data0/d, /data1/d ]~g" /etc/cassandra/cassandra.yaml
else 
  sudo sed -i -e "s~data_file_directories:[^\n]*~data_file_directories: [ /data0/d ]~g" /etc/cassandra/cassandra.yaml
fi

# Commitlog on attached volume. Comment out to store commitlog on the ephemeral instance volume at /var/lib/cassandra/commitlog.
#sudo sed -i -e "s~/var/lib/cassandra/commitlog~/data/c~g" /etc/cassandra/cassandra.yaml
#sudo sed -i -e "s~/var/lib/cassandra/commitlog~/mnt/ramdisk/commitlog~g" /etc/cassandra/cassandra.yaml
#sudo sed -i -e "s~/var/lib/cassandra/commitlog~~g" /etc/cassandra/cassandra.yaml
sudo sed -i -e "s~commitlog_directory:[^\n]*~commitlog_directory: /data0/c~g" /etc/cassandra/cassandra.yaml

# Minimal number of vnodes, we do not need elasticity
sudo sed -i -e "s~num_tokens:[ 0-9]*~num_tokens: 1~g" /etc/cassandra/cassandra.yaml

# No redundancy
sudo sed -i -e "s~allocate_tokens_for_local_replication_factor: [ 0-9]*~allocate_tokens_for_local_replication_factor: 1~g" /etc/cassandra/cassandra.yaml

# If provided, use initial token list to decrease cluster starting time
if [ "$INITIAL_TOKEN" != "" ]; then
  sudo sed -i -e "s~[ #]*initial_token:[^\n]*~initial_token: $INITIAL_TOKEN~g" /etc/cassandra/cassandra.yaml
fi

# In test env, give enough time to Cassandra coordinator to complete the write (cassandra.yaml write_request_timeout_in_ms)
# so there is no doubt that coordinator is the bottleneck,
# and make sure client time out is more (not equal) than that to avoid gocql error "no response received from cassandra within timeout period".
# In prod environments, increasing write_request_timeout_in_ms and corresponding client timeout is not a solution.
sudo sed -i -e "s~write_request_timeout_in_ms:[ ]*[0-9]*~write_request_timeout_in_ms: 10000~g" /etc/cassandra/cassandra.yaml

# Experimenting with key cache size
# Default is 5% of the heap 2000-100mb>, make it bigger (does not help)
# sudo sed -i -e "s~key_cache_size_in_mb:[ 0-9]*~key_cache_size_in_mb: 1000~g" /etc/cassandra/cassandra.yaml
# Do not store keys longer than 120s (does not help)
#sudo sed -i -e "s~key_cache_save_period:[ 0-9]*~key_cache_save_period: 120~g" /etc/cassandra/cassandra.yaml

sudo rm -fR /var/lib/cassandra/data/*
sudo rm -fR /var/lib/cassandra/commitlog/*
if [ ! -d "/data0" ]; then
  sudo rm -fR /data0/*
fi
if [ ! -d "/data1" ]; then
  sudo rm -fR /data1/*
fi
sudo rm -fR /var/lib/cassandra/saved_caches/*

# To avoid "Cannot start node if snitch’s data center (dc1) differs from previous data center (datacenter1)"
# error, keep using dc and rack variables as they are (dc1,rack1) in /etc/cassandra/cassandra-rackdc.properties
# but ignore the dc - it's a testing env
echo 'JVM_OPTS="$JVM_OPTS -Dcassandra.ignore_dc=true"' | sudo tee -a /etc/cassandra/cassandra-env.sh

# We do not need this config file, delete it
sudo rm -f rm /etc/cassandra/cassandra-topology.properties

# No need to logrotate, Cassandra uses log4j, configure it conservatively
sudo sed -i -e "s~<maxFileSize>[^<]*</maxFileSize>~<maxFileSize>10MB</maxFileSize>~g" /etc/cassandra/logback.xml
sudo sed -i -e "s~<totalSizeCap>[^<]*</totalSizeCap>~<totalSizeCap>1GB</totalSizeCap>~g" /etc/cassandra/logback.xml

mount_device(){
	local mount_dir="/data"$1
 	local device_name=$2
    echo Mounting $device_name at $mount_dir
    if [ "$(lsblk -f | grep -E $device_name'[ ]+xfs')" == "" ]; then
      echo Formatting partition
	    sudo mkfs -t xfs /dev/$device_name
    else
      echo Partition already formatted
    fi
    if [ ! -d "$mount_dir" ]; then
      echo Creating $mount_dir
	  sudo mkdir $mount_dir
    else
      echo $mount_dir already created
    fi
    if [ "$(lsblk -f | grep $mount_dir)" == "" ]; then
      echo Mounting...
	  sudo mount /dev/$device_name $mount_dir
    else
      echo Already mounted
    fi
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

sudo systemctl start cassandra
if [ "$?" -ne "0" ]; then
    echo Cannot start cassandra, exiting
    exit $?
fi
