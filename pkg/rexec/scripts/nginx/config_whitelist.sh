if [ "$BASTION_ALLOWED_IPS" = "" ]; then
  echo Error, missing: BASTION_ALLOWED_IPS=1.2.3.4/24,5.6.7.8/16
  exit 1
fi

if [ ! -d "/etc/nginx/includes" ]; then
  sudo mkdir /etc/nginx/includes
fi

WHITELIST_CONFIG_FILE=/etc/nginx/includes/allowed_ips.conf

if [ -f "$WHITELIST_CONFIG_FILE" ]; then
  sudo rm $WHITELIST_CONFIG_FILE
fi
sudo touch $WHITELIST_CONFIG_FILE

IFS=',' read -ra CIDR <<< "$BASTION_ALLOWED_IPS"
for i in "${CIDR[@]}"; do
  echo "allow $i;" | sudo tee -a $WHITELIST_CONFIG_FILE
done
echo "deny all;" | sudo tee -a $WHITELIST_CONFIG_FILE

