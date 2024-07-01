sudo DEBIAN_FRONTEND=noninteractive add-apt-repository -y ppa:rabbitmq/rabbitmq-erlang
sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

# Erlang from https://launchpad.net/~rabbitmq/+archive/ubuntu/rabbitmq-erlang
ERLANG_VER=1:26.2.5-1rmq1ppa1~ubuntu24.04.1
sudo DEBIAN_FRONTEND=noninteractive apt-get -y install erlang-base=$ERLANG_VER \
    erlang-asn1=$ERLANG_VER erlang-crypto=$ERLANG_VER erlang-eldap=$ERLANG_VER erlang-ftp=$ERLANG_VER erlang-inets=$ERLANG_VER \
    erlang-mnesia=$ERLANG_VER erlang-os-mon=$ERLANG_VER erlang-parsetools=$ERLANG_VER erlang-public-key=$ERLANG_VER \
    erlang-runtime-tools=$ERLANG_VER erlang-snmp=$ERLANG_VER erlang-ssl=$ERLANG_VER \
    erlang-syntax-tools=$ERLANG_VER erlang-tftp=$ERLANG_VER erlang-tools=$ERLANG_VER erlang-xmerl=$ERLANG_VER

# RabbitMQ server
RABBITMQ_VER=3.12.1-1ubuntu1
sudo DEBIAN_FRONTEND=noninteractive apt-get -y --fix-missing install rabbitmq-server=$RABBITMQ_VER

# https://www.cherryservers.com/blog/how-to-install-and-start-using-rabbitmq-on-ubuntu-22-04

# sudo DEBIAN_FRONTEND=noninteractive apt-get -y install gnupg apt-transport-https

# curl -1sLf "https://keys.openpgp.org/vks/v1/by-fingerprint/0A9AF2115F4687BD29803A206B73A36E6026DFCA" | sudo gpg --dearmor | sudo tee /usr/share/keyrings/com.rabbitmq.team.gpg > /dev/null
# curl -1sLf "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xf77f1eda57ebb1cc" | sudo gpg --dearmor | sudo tee /usr/share/keyrings/net.launchpad.ppa.rabbitmq.erlang.gpg > /dev/null
# curl -1sLf "https://packagecloud.io/rabbitmq/rabbitmq-server/gpgkey" | sudo gpg --dearmor | sudo tee /usr/share/keyrings/io.packagecloud.rabbitmq.gpg > /dev/null

# # Use RabbitMQ "jammy" release for Ubuntu 22.04:
# sudo tee /etc/apt/sources.list.d/rabbitmq.list <<EOF
# deb [signed-by=/usr/share/keyrings/net.launchpad.ppa.rabbitmq.erlang.gpg] http://ppa.launchpad.net/rabbitmq/rabbitmq-erlang/ubuntu jammy main
# deb-src [signed-by=/usr/share/keyrings/net.launchpad.ppa.rabbitmq.erlang.gpg] http://ppa.launchpad.net/rabbitmq/rabbitmq-erlang/ubuntu jammy main
# deb [signed-by=/usr/share/keyrings/io.packagecloud.rabbitmq.gpg] https://packagecloud.io/rabbitmq/rabbitmq-server/ubuntu/ jammy main
# deb-src [signed-by=/usr/share/keyrings/io.packagecloud.rabbitmq.gpg] https://packagecloud.io/rabbitmq/rabbitmq-server/ubuntu/ jammy main
# EOF

# # Erlang
# sudo DEBIAN_FRONTEND=noninteractive apt-get -y install erlang-base \
#     erlang-asn1 erlang-crypto erlang-eldap erlang-ftp erlang-inets \
#     erlang-mnesia erlang-os-mon erlang-parsetools erlang-public-key \
#     erlang-runtime-tools erlang-snmp erlang-ssl \
#     erlang-syntax-tools erlang-tftp erlang-tools erlang-xmerl

# # RabbitMQ server
# sudo DEBIAN_FRONTEND=noninteractive apt-get -y --fix-missing install rabbitmq-server




# https://www.rabbitmq.com/docs/install-debian#apt-quick-start-cloudsmith

# sudo DEBIAN_FRONTEND=noninteractive apt-get install curl gnupg apt-transport-https -y

# ## Team RabbitMQ's main signing key
# curl -1sLf "https://keys.openpgp.org/vks/v1/by-fingerprint/0A9AF2115F4687BD29803A206B73A36E6026DFCA" | sudo gpg --dearmor | sudo tee /usr/share/keyrings/com.rabbitmq.team.gpg > /dev/null
# ## Community mirror of Cloudsmith: modern Erlang repository
# curl -1sLf https://github.com/rabbitmq/signing-keys/releases/download/3.0/cloudsmith.rabbitmq-erlang.E495BB49CC4BBE5B.key | sudo gpg --dearmor | sudo tee /usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg > /dev/null
# ## Community mirror of Cloudsmith: RabbitMQ repository
# curl -1sLf https://github.com/rabbitmq/signing-keys/releases/download/3.0/cloudsmith.rabbitmq-server.9F4587F226208342.key | sudo gpg --dearmor | sudo tee /usr/share/keyrings/rabbitmq.9F4587F226208342.gpg > /dev/null

# ## Add apt repositories maintained by Team RabbitMQ
# sudo tee /etc/apt/sources.list.d/rabbitmq.list <<EOF
# ## Provides modern Erlang/OTP releases
# ##
# deb [signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa1.novemberain.com/rabbitmq/rabbitmq-erlang/deb/ubuntu jammy main
# deb-src [signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa1.novemberain.com/rabbitmq/rabbitmq-erlang/deb/ubuntu jammy main

# # another mirror for redundancy
# deb [signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa2.novemberain.com/rabbitmq/rabbitmq-erlang/deb/ubuntu jammy main
# deb-src [signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa2.novemberain.com/rabbitmq/rabbitmq-erlang/deb/ubuntu jammy main

# ## Provides RabbitMQ
# ##
# deb [signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa1.novemberain.com/rabbitmq/rabbitmq-server/deb/ubuntu jammy main
# deb-src [signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa1.novemberain.com/rabbitmq/rabbitmq-server/deb/ubuntu jammy main

# # another mirror for redundancy
# deb [signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa2.novemberain.com/rabbitmq/rabbitmq-server/deb/ubuntu jammy main
# deb-src [signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa2.novemberain.com/rabbitmq/rabbitmq-server/deb/ubuntu jammy main
# EOF

# ## Update package indices
# sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

# ## Install Erlang packages
# sudo DEBIAN_FRONTEND=noninteractive apt-get install -y erlang-base \
#                         erlang-asn1 erlang-crypto erlang-eldap erlang-ftp erlang-inets \
#                         erlang-mnesia erlang-os-mon erlang-parsetools erlang-public-key \
#                         erlang-runtime-tools erlang-snmp erlang-ssl \
#                         erlang-syntax-tools erlang-tftp erlang-tools erlang-xmerl

# ## Install rabbitmq-server and its dependencies
# sudo DEBIAN_FRONTEND=noninteractive apt-get install rabbitmq-server -y --fix-missing






#From launchpad https://www.rabbitmq.com/docs/install-debian#apt-launchpad-erlang

# curl -1sLf "https://github.com/rabbitmq/signing-keys/releases/download/3.0/rabbitmq-release-signing-key.asc" | sudo gpg --dearmor | sudo tee /usr/share/keyrings/com.github.rabbitmq.signing.gpg > /dev/null
# curl -1sLf "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xf77f1eda57ebb1cc" | sudo gpg --dearmor | sudo tee /usr/share/keyrings/net.launchpad.ppa.rabbitmq.erlang.gpg > /dev/null

# sudo DEBIAN_FRONTEND=noninteractive apt-get install apt-transport-https

# sudo tee /etc/apt/sources.list.d/rabbitmq.list <<EOF
# deb [signed-by=/usr/share/keyrings/net.launchpad.ppa.rabbitmq.erlang.gpg] http://ppa.launchpad.net/rabbitmq/rabbitmq-erlang/ubuntu jammy main
# deb-src [signed-by=/usr/share/keyrings/net.launchpad.ppa.rabbitmq.erlang.gpg] http://ppa.launchpad.net/rabbitmq/rabbitmq-erlang/ubuntu jammy main
# EOF

# sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

# sudo DEBIAN_FRONTEND=noninteractive apt-get install -y erlang-base \
#                         erlang-asn1 erlang-crypto erlang-eldap erlang-ftp erlang-inets \
#                         erlang-mnesia erlang-os-mon erlang-parsetools erlang-public-key \
#                         erlang-runtime-tools erlang-snmp erlang-ssl \
#                         erlang-syntax-tools erlang-tftp erlang-tools erlang-xmerl

# sudo DEBIAN_FRONTEND=noninteractive apt-get install rabbitmq-server -y --fix-missing