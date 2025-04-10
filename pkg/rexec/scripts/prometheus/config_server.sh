# Prometheus server (assuming node exporter also running on it)
# https://www.digitalocean.com/community/tutorials/how-to-install-prometheus-on-ubuntu-16-04

if [ "$PROMETHEUS_TARGETS" = "" ]; then
  echo "Error, missing: PROMETHEUS_TARGETS=\'localhost:9100\',\'10.5.1.10:9100\'"
 exit 1
fi

sudo systemctl stop prometheus 2>/dev/null

PROMETHEUS_YAML_FILE=/etc/prometheus/prometheus.yml

sudo rm -f $PROMETHEUS_YAML_FILE

sudo tee $PROMETHEUS_YAML_FILE <<EOF
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: 'prometheus'
    scrape_interval: 5s
    static_configs:
      - targets: ['localhost:9090']
  - job_name: 'node_exporter'
    scrape_interval: 5s
    static_configs:
      - targets: [$PROMETHEUS_TARGETS]
EOF
sudo chown -R prometheus:prometheus $PROMETHEUS_YAML_FILE

PROMETHEUS_SERVICE_FILE=/etc/systemd/system/prometheus.service

sudo rm -f $PROMETHEUS_SERVICE_FILE

sudo tee $PROMETHEUS_SERVICE_FILE <<EOF
[Unit] 
Description=Prometheus server
Wants=network-online.target
After=network-online.target
[Service]
User=prometheus
Group=prometheus
Type=simple
ExecStart=/usr/local/bin/prometheus --config.file /etc/prometheus/prometheus.yml --storage.tsdb.path /var/lib/prometheus/ --storage.tsdb.retention.time=1d
[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload

sudo systemctl start prometheus
sudo systemctl status prometheus

curl -s http://localhost:9090
if [ "$?" -ne "0" ]; then
    echo Cannot check localhost:9090
    exit $?
fi
