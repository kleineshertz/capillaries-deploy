if [ "$CASSANDRA_HOSTS" = "" ]; then
  echo Error, missing: CASSANDRA_HOSTS='["10.5.0.11","10.5.0.12","10.5.0.13"]'
  exit 1
fi
if [ "$AMQP_URL" = "" ]; then
  echo Error, missing: AMQP_URL=amqp://guest:guest@10.5.0.5/
  exit 1
fi
if [ "$INTERNAL_WEBAPI_PORT" = "" ]; then
  echo Error, missing: INTERNAL_WEBAPI_PORT=6543
  exit 1
fi
if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

if [ "$EXTERNAL_IP_ADDRESS" = "" ]; then
  echo Error, missing EXTERNAL_IP_ADDRESS=1.2.3.4
  exit 1
fi

pkill -2 capiwebapi
processid=$(pgrep capiwebapi)
if [ "$processid" != "" ]; then
  pkill -9 capiwebapi
fi

# If we ever use https and/or domain names, or use other port than 80, revisit this piece.
WEBAPI_ACCESS_CONTROL_ACCESS_ORIGIN="http://$EXTERNAL_IP_ADDRESS"

ENV_CONFIG_FILE=/home/$SSH_USER/bin/capiwebapi.json

echo "Patching webapi connectivity config"
sed -i -e 's~"url":[ ]*"[a-zA-Z0-9@\.:\/\-_$ ]*"~"url": "'"$AMQP_URL"'"~g' $ENV_CONFIG_FILE
sed -i -e 's~"hosts\":[ ]*\[[0-9a-zA-Z\.\,\-_ "]*\]~"hosts": '"$CASSANDRA_HOSTS~g" $ENV_CONFIG_FILE
sed -i -e 's~"webapi_port\":[ ]*[0-9]*~"webapi_port": '$INTERNAL_WEBAPI_PORT'~g' $ENV_CONFIG_FILE
sed -i -e 's~"access_control_allow_origin\":[ ]*"[0-9a-zA-Z\,\.:\/\-_"]*"~"access_control_allow_origin": "'"$WEBAPI_ACCESS_CONTROL_ACCESS_ORIGIN"'"~g' $ENV_CONFIG_FILE
sed -i -e "s~\"keyspace_replication_config\":[ ]*\"[^\"]*\"~\"keyspace_replication_config\": \"{'class':'SimpleStrategy', 'replication_factor':1}\"~g" $ENV_CONFIG_FILE

#echo "Patching config to use ca at /home/"$SSH_USER"/ca"
#sed -i -e 's~"ca_path":[ ]*"[^\"]*"~"ca_path":"/home/'$SSH_USER'/ca"~g' $ENV_CONFIG_FILE
# If you want to use Ubuntu CA store:
#sed -i -e 's~"ca_path":[ ]*"[a-zA-Z0-9\.\/\-_]*"~"ca_path":"/usr/local/share/ca-certificates"~g' $ENV_CONFIG_FILE

sudo chmod 744 /home/$SSH_USER/bin/capiwebapi
/home/$SSH_USER/bin/capiwebapi >> /mnt/capi_log/capiwebapi.log 2>&1 &

