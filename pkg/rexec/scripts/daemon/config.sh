# Make it as idempotent as possible, it can be called over and over

if [ "$CASSANDRA_HOSTS" = "" ]; then
  echo Error, missing: CASSANDRA_HOSTS='["10.5.0.11","10.5.0.12","10.5.0.13"]'
  exit 1
fi
if [ "$AMQP_URL" = "" ]; then
  echo Error, missing: AMQP_URL=amqp://guest:guest@10.5.0.5/
  exit 1
fi
if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

pkill -2 capidaemon
processid=$(pgrep capidaemon)
if [ "$processid" != "" ]; then
  echo Trying pkill -9...
  pkill -9 capidaemon 2> /dev/null
  processid=$(pgrep capidaemon)
  if [ "$processid" != "" ]; then
    echo pkill -9 did not kill
    exit 9
  fi 
fi

ENV_CONFIG_FILE=/home/$SSH_USER/bin/capidaemon.json

sed -i -e 's~"url":[ ]*"[a-zA-Z0-9@\.:\/\-_$ ]*"~"url": "'"$AMQP_URL"'"~g' $ENV_CONFIG_FILE
sed -i -e 's~"hosts":[ ]*\[[0-9a-zA-Z\.\,\-_ "]*\]~"hosts": '$CASSANDRA_HOSTS"~g" $ENV_CONFIG_FILE
sed -i -e 's~"python_interpreter_path":[ ]*"[a-zA-Z0-9]*"~"python_interpreter_path": "python3"~g' $ENV_CONFIG_FILE
sed -i -e 's~"level":[ ]*"[a-zA-Z]*"~"level": "info"~g' $ENV_CONFIG_FILE

echo "Patching config to use ca at /home/"$SSH_USER"/ca"
sed -i -e 's~"ca_path":[ ]*"[^\"]*"~"ca_path":"/home/'$SSH_USER'/ca"~g' $ENV_CONFIG_FILE
# If you want to use Ubuntu CA store:
#sed -i -e 's~"ca_path":[ ]*"[a-zA-Z0-9\.\/\-_]*"~"ca_path":"/usr/local/share/ca-certificates"~g' $ENV_CONFIG_FILE


# For our perf testing purposes, decrease latency at the expense of the message queue load
# sed -i -e 's~"dead_letter_ttl":[ ]*[0-9]*~"dead_letter_ttl": 100~g' $ENV_CONFIG_FILE

# If you use your test Cassandra setup up to the limit, try to avoid "Operation timed out - received only 0 responses"
# Make replication factor at least 2 to make reads more available, 1 for faster writes
# https://stackoverflow.com/questions/38231621/cassandra-operation-timed-out
sed -i -e "s~\"keyspace_replication_config\":[ ]*\"[^\"]*\"~\"keyspace_replication_config\": \"{'class':'SimpleStrategy', 'replication_factor':1}\"~g" $ENV_CONFIG_FILE

# In test env, give enough time to Cassandra coordinator to complete the write (cassandra.yaml write_request_timeout_in_ms)
# so there is no doubt that coordinator is the bottleneck,
# and make sure client time out is more (not equal) than that to avoid gocql error "no response received from cassandra within timeout period".
# In prod environments, increasing write_request_timeout_in_ms and corresponding client timeout is not a solution.
sed -i -e "s~\"timeout\":[ ]*[0-9]*~\"timeout\": 15000~g" $ENV_CONFIG_FILE

# Default number writer workers may be pretty aggressive,
# watch for "Operation timed out - received only 0 responses" on writes, throttle it down to 10 or lower if needed
if [ "$DAEMON_DB_WRITERS" != "" ]; then
  sed -i -e "s~\"writer_workers\":[ 0-9]*~\"writer_workers\": $DAEMON_DB_WRITERS~g" $ENV_CONFIG_FILE
fi

# Thread pool size - number of workers handling RabbitMQ messages - is about using daemon instance CPU resources
if [ "$DAEMON_THREAD_POOL_SIZE" != "" ]; then
  sed -i -e "s~\"thread_pool_size\":[ ]*[0-9]*~\"thread_pool_size\": $DAEMON_THREAD_POOL_SIZE~g" $ENV_CONFIG_FILE
fi

sudo rm -fR /var/log/capidaemon
sudo mkdir /var/log/capidaemon
sudo chmod 777 /var/log/capidaemon
sudo chmod 744 /home/$SSH_USER/bin/capidaemon

/home/$SSH_USER/bin/capidaemon >> /var/log/capidaemon/capidaemon.log 2>&1 &

