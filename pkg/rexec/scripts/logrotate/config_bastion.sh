# Make it as idempotent as possible, it can be called over and over

# Logrotate
LOGROTATE_CONFIG_FILE=/etc/logrotate.d/capidaemon_logrotate.conf

sudo rm -f $LOGROTATE_CONFIG_FILE
sudo tee $LOGROTATE_CONFIG_FILE <<EOF
/mnt/capi_log/*.log {
    create 0644 root root
    hourly
    rotate 10
    missingok
    notifempty
    compress
    sharedscripts
    postrotate
        echo "Log rotated" > /mnt/capi_log
    endscript
}
EOF

sudo systemctl restart logrotate

# Logrotate/Cron
# Make sure less /etc/cron.daily/logrotate has something like this (should be installed by logrotate installer):
# #!/bin/sh
# /usr/sbin/logrotate -s /var/lib/logrotate/logrotate.status /etc/logrotate.conf
# EXITVALUE=$?
# if [ $EXITVALUE != 0 ]; then
#     /usr/bin/logger -t logrotate "ALERT exited abnormally with [$EXITVALUE]"
# fi
# exit 0
