# https://www.digitalocean.com/community/tutorials/how-to-configure-nginx-as-a-reverse-proxy-on-ubuntu-22-04

# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y nginx 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo nginx install error, exiting
    exit $?
fi

# Remove nginx stub site
sudo rm -f /etc/nginx/sites-enabled/default

# nginx has a habit to write "syntax is ok" to stderr. Ignore it and rely on the exit code
sudo nginx -t 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo nginx config error, exiting
    exit $?
fi

sudo systemctl restart nginx