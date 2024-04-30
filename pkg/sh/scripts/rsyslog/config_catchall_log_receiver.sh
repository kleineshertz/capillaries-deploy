# Override AppArmor that prevents rsyslog from writing to /mnt/capi_log
if ! grep -q "/mnt/capi_log" /etc/apparmor.d/usr.sbin.rsyslogd; then
  sudo sed -i -E "s~/var/log/\*\*[ ]+rw~/var/log/** rw,\n  /mnt/capi_log/** rw~g" /etc/apparmor.d/usr.sbin.rsyslogd
fi

RSYSLOG_CONFIG_FILE=/etc/rsyslog.d/capillaries_catchall_receiver.conf

sudo rm -f $RSYSLOG_CONFIG_FILE

sudo tee $RSYSLOG_CONFIG_FILE <<EOF
module(load="imudp")
template (name="CassandraFormat" type="string" string="%fromhost% %msg%\n")
ruleset(name="catchall"){
    if \$syslogtag == 'capidaemon' then {
        action(type="omfile" DirCreateMode="0777" FileCreateMode="0644" file="/mnt/capi_log/capidaemon.log")
    } else if \$syslogtag == 'cassandra_system' then {
        action(type="omfile" DirCreateMode="0777" FileCreateMode="0644" file="/mnt/capi_log/cassandra_system.log" template="CassandraFormat")
    } else if \$syslogtag == 'cassandra_debug' then {
        action(type="omfile" DirCreateMode="0777" FileCreateMode="0644" file="/mnt/capi_log/cassandra_debug.log" template="CassandraFormat")
    } else if \$syslogtag == 'rabbitmq' then {
        action(type="omfile" DirCreateMode="0777" FileCreateMode="0644" file="/mnt/capi_log/rabbitmq.log")
    } else if \$syslogtag == 'prometheus' then {
        action(type="omfile" DirCreateMode="0777" FileCreateMode="0644" file="/mnt/capi_log/prometheus.log")
    } else {
        action(type="omfile" DirCreateMode="0777" FileCreateMode="0644" file="/mnt/capi_log/other.log")
    }
}
input(type="imudp" port="514" ruleset="catchall")
EOF

sudo systemctl restart rsyslog
