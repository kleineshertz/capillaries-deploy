RSYSLOG_CONFIG_FILE=/etc/rsyslog.d/casandra_receiver.conf

sudo rm -f $RSYSLOG_CONFIG_FILE

sudo tee $RSYSLOG_CONFIG_FILE <<EOF
module(load="imudp")
ruleset(name="catchall"){action(type="omfile" DirCreateMode="0777" FileCreateMode="0644" file="/mnt/capi_log/$syslogtag-$fromhost-ip.log")}
input(type="imudp" port="514" ruleset="catchall")
EOF

sudo systemctl restart rsyslog
