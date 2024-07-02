if [ "$SSH_USER" = "" ]; then
  echo Error, missing: SSH_USER=ubuntu
  exit 1
fi

UI_CONFIG_FILE=/etc/nginx/sites-available/ui
if [ -f "$UI_CONFIG_FILE" ]; then
  sudo rm -f $UI_CONFIG_FILE
fi

sudo tee $UI_CONFIG_FILE <<EOF
server {
  listen 80;
  listen [::]:80;
  root /home/$SSH_USER/ui;
  index index.html;
  location / {
    include includes/allowed_ips.conf;
  }
}
EOF

sudo chmod 755 /home
sudo chmod 755 /home/$SSH_USER
sudo chmod 755 /home/$SSH_USER/ui

if [ ! -L "/etc/nginx/sites-enabled/ui" ]; then
  sudo ln -s $UI_CONFIG_FILE /etc/nginx/sites-enabled/
fi

# nginx has a habit to write "syntax is ok" to stderr. Ignore it and rely on the exit code
sudo nginx -t 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo nginx config error, exiting
    exit $?
fi

sudo systemctl restart nginx