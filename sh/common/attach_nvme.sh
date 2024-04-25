# It requires cassandra user, so do not run it before you install Cassandra
if [ "$NVME_REGEX" = "" ]; then
  echo Error, missing: NVME_REGEX="nvme[0-9]n[0-9] 558.8G"
  exit 1
fi

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
