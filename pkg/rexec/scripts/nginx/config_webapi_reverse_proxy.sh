# nginx reverse proxy
# https://www.digitalocean.com/community/tutorials/how-to-configure-nginx-as-a-reverse-proxy-on-ubuntu-22-04

if [ "$INTERNAL_WEBAPI_PORT" = "" ]; then
  echo Error, missing: INTERNAL_WEBAPI_PORT=6543
  exit 1
fi
if [ "$EXTERNAL_WEBAPI_PORT" = "" ]; then
  echo Error, missing: EXTERNAL_WEBAPI_PORT=6544
  exit 1
fi

CONFIG_FILE=/etc/nginx/sites-available/webapi
if [ -f "$CONFIG_FILE" ]; then
  sudo rm -f $CONFIG_FILE
fi


sudo tee $CONFIG_FILE <<EOF
server {
    listen $EXTERNAL_WEBAPI_PORT;
    location / {
        proxy_pass http://localhost:$INTERNAL_WEBAPI_PORT;
        include proxy_params;
        include includes/allowed_ips.conf;
    }
}
EOF

if [ ! -L "/etc/nginx/sites-enabled/webapi" ]; then
  sudo ln -s $CONFIG_FILE /etc/nginx/sites-enabled/
fi

# nginx has a habit to write "syntax is ok" to stderr. Ignore it and rely on the exit code
sudo nginx -t 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo nginx config error, exiting
    exit $?
fi

sudo systemctl restart nginx