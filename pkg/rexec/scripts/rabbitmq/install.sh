if [ "$RABBITMQ_ERLANG_VERSION_AMD64" = "" ]; then
  echo Error, missing: RABBITMQ_ERLANG_VERSION_AMD64=...
  exit 1
fi
if [ "$RABBITMQ_SERVER_VERSION_AMD64" = "" ]; then
  echo Error, missing: RABBITMQ_SERVER_VERSION_AMD64=...
  exit 1
fi
if [ "$RABBITMQ_ERLANG_VERSION_ARM64" = "" ]; then
  echo Error, missing: RABBITMQ_ERLANG_VERSION_ARM64=...
  exit 1
fi
if [ "$RABBITMQ_SERVER_VERSION_ARM64" = "" ]; then
  echo Error, missing: RABBITMQ_SERVER_VERSION_ARM64=...
  exit 1
fi

sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y curl gnupg 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo gnugpg install error, exiting
    exit $?
fi

# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo apt-transport-https install error, exiting
    exit $?
fi

## Team RabbitMQ's main signing key
curl -1sLf "https://keys.openpgp.org/vks/v1/by-fingerprint/0A9AF2115F4687BD29803A206B73A36E6026DFCA" | sudo gpg --dearmor | sudo tee /usr/share/keyrings/com.rabbitmq.team.gpg > /dev/null
## Community mirror of Cloudsmith: modern Erlang repository
curl -1sLf https://github.com/rabbitmq/signing-keys/releases/download/3.0/cloudsmith.rabbitmq-erlang.E495BB49CC4BBE5B.key | sudo gpg --dearmor | sudo tee /usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg > /dev/null
## Community mirror of Cloudsmith: RabbitMQ repository
curl -1sLf https://github.com/rabbitmq/signing-keys/releases/download/3.0/cloudsmith.rabbitmq-server.9F4587F226208342.key | sudo gpg --dearmor | sudo tee /usr/share/keyrings/rabbitmq.9F4587F226208342.gpg > /dev/null


sudo tee /etc/apt/sources.list.d/rabbitmq.list <<EOF
## Provides modern Erlang/OTP releases from a Cloudsmith mirror
##
deb [arch=amd64 signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa1.rabbitmq.com/rabbitmq/rabbitmq-erlang/deb/ubuntu noble main
deb-src [signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa1.rabbitmq.com/rabbitmq/rabbitmq-erlang/deb/ubuntu noble main

# another mirror for redundancy
deb [arch=amd64 signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa2.rabbitmq.com/rabbitmq/rabbitmq-erlang/deb/ubuntu noble main
deb-src [signed-by=/usr/share/keyrings/rabbitmq.E495BB49CC4BBE5B.gpg] https://ppa2.rabbitmq.com/rabbitmq/rabbitmq-erlang/deb/ubuntu noble main

## Provides RabbitMQ from a Cloudsmith mirror
##
deb [arch=amd64 signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa1.rabbitmq.com/rabbitmq/rabbitmq-server/deb/ubuntu noble main
deb-src [signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa1.rabbitmq.com/rabbitmq/rabbitmq-server/deb/ubuntu noble main

# another mirror for redundancy
deb [arch=amd64 signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa2.rabbitmq.com/rabbitmq/rabbitmq-server/deb/ubuntu noble main
deb-src [signed-by=/usr/share/keyrings/rabbitmq.9F4587F226208342.gpg] https://ppa2.rabbitmq.com/rabbitmq/rabbitmq-server/deb/ubuntu noble main
EOF

sudo DEBIAN_FRONTEND=noninteractive apt-get update -y

# See available packages:

# apt list -a erlang-base

# As of Dec 2024:
# erlang-base/noble,noble 1:27.2-1 amd64
# erlang-base/noble,noble 1:27.1.3-1 amd64
# erlang-base/noble,noble 1:27.1.2-1 amd64
# erlang-base/noble,noble 1:26.2.5.6-1 amd64
# erlang-base/noble,noble 1:26.2.5.5-1 amd64
# erlang-base/noble,noble 1:26.2.5.4-1 amd64
# erlang-base/noble,now 1:25.3.2.8+dfsg-1ubuntu4 arm64 [installed]

# As of April 2025:
# erlang-base/noble,noble 1:27.3.1-1 amd64
# erlang-base/noble,noble 1:27.3-1 amd64
# erlang-base/noble,noble 1:27.2.4-1 amd64
# erlang-base/noble,noble 1:27.2.3-1 amd64
# erlang-base/noble,noble 1:27.2.2-1 amd64
# erlang-base/noble,noble 1:27.2.1-1 amd64
# erlang-base/noble,noble 1:27.2-1 amd64
# erlang-base/noble,noble 1:27.1.3-1 amd64
# erlang-base/noble,noble 1:27.1.2-1 amd64
# erlang-base/noble,noble 1:26.2.5.10-1 amd64
# erlang-base/noble,noble 1:26.2.5.9-1 amd64
# erlang-base/noble,noble 1:26.2.5.8-1 amd64
# erlang-base/noble,noble 1:26.2.5.7-1 amd64
# erlang-base/noble,noble 1:26.2.5.6-1 amd64
# erlang-base/noble,noble 1:26.2.5.5-1 amd64
# erlang-base/noble,noble 1:26.2.5.4-1 amd64
# erlang-base/noble-updates,noble-security,now 1:25.3.2.8+dfsg-1ubuntu4.1 arm64 [installed]
# erlang-base/noble 1:25.3.2.8+dfsg-1ubuntu4 arm64

# As of April 2025
# erlang-base/noble,noble 1:27.3.2-1 amd64
# erlang-base/noble,noble 1:27.3.1-1 amd64
# erlang-base/noble,noble 1:27.3-1 amd64
# erlang-base/noble,noble 1:27.2.4-1 amd64
# erlang-base/noble,noble 1:27.2.3-1 amd64
# erlang-base/noble,noble 1:27.2.2-1 amd64
# erlang-base/noble,noble 1:27.2.1-1 amd64
# erlang-base/noble,noble 1:27.2-1 amd64
# erlang-base/noble,noble 1:27.1.3-1 amd64
# erlang-base/noble,noble 1:27.1.2-1 amd64
# erlang-base/noble,noble 1:26.2.5.10-1 amd64
# erlang-base/noble,noble 1:26.2.5.9-1 amd64
# erlang-base/noble,noble 1:26.2.5.8-1 amd64
# erlang-base/noble,noble 1:26.2.5.7-1 amd64
# erlang-base/noble,noble 1:26.2.5.6-1 amd64
# erlang-base/noble,noble 1:26.2.5.5-1 amd64
# erlang-base/noble,noble 1:26.2.5.4-1 amd64
# erlang-base/noble-updates,noble-security 1:25.3.2.8+dfsg-1ubuntu4.2 arm64 [upgradable from: 1:25.3.2.8+dfsg-1ubuntu4]
# erlang-base/noble,now 1:25.3.2.8+dfsg-1ubuntu4 arm64 [installed,upgradable to: 1:25.3.2.8+dfsg-1ubuntu4.2]

# apt list -a rabbitmq-server

# As of Dec 2024:
# rabbitmq-server/noble,noble 4.0.5-1 all [upgradable from: 3.12.1-1ubuntu1]
# rabbitmq-server/noble,noble 4.0.4-1 all
# rabbitmq-server/noble,noble 4.0.3-1 all
# rabbitmq-server/noble,noble 4.0.2-1 all
# rabbitmq-server/noble,noble 4.0.1-1 all
# rabbitmq-server/noble,noble 4.0.0-1 all
# rabbitmq-server/noble,noble 3.13.7-1 all
# rabbitmq-server/noble,noble 3.13.6-1 all
# rabbitmq-server/noble,noble 3.13.5-1 all
# rabbitmq-server/noble,noble 3.13.4-1 all
# rabbitmq-server/noble,noble 3.12.14-1 all
# rabbitmq-server/noble,now 3.12.1-1ubuntu1 all [installed,upgradable to: 4.0.5-1]

# As of April 2025:
# rabbitmq-server/noble,noble 4.0.7-1 all [upgradable from: 3.12.1-1ubuntu1]
# rabbitmq-server/noble,noble 4.0.6-1 all
# rabbitmq-server/noble,noble 4.0.5-1 all
# rabbitmq-server/noble,noble 4.0.4-1 all
# rabbitmq-server/noble,noble 4.0.3-1 all
# rabbitmq-server/noble,noble 4.0.2-1 all
# rabbitmq-server/noble,noble 4.0.1-1 all
# rabbitmq-server/noble,noble 4.0.0-1 all
# rabbitmq-server/noble,noble 3.13.7-1 all
# rabbitmq-server/noble,noble 3.13.6-1 all
# rabbitmq-server/noble,noble 3.13.5-1 all
# rabbitmq-server/noble,noble 3.13.4-1 all
# rabbitmq-server/noble,noble 3.12.14-1 all
# rabbitmq-server/noble-updates,noble-security 3.12.1-1ubuntu1.2 all
# rabbitmq-server/noble,now 3.12.1-1ubuntu1 all [installed,upgradable to: 4.0.7-1]

# Compatibility chart: https://www.rabbitmq.com/docs/which-erlang and https://www.rabbitmq.com/docs/3.13/which-erlang

if [ "$(uname -p)" == "x86_64" ]; then
export ERLANG_VER=$RABBITMQ_ERLANG_VERSION_AMD64
export RABBITMQ_VER=$RABBITMQ_SERVER_VERSION_AMD64
else
export ERLANG_VER=$RABBITMQ_ERLANG_VERSION_ARM64
export RABBITMQ_VER=$RABBITMQ_SERVER_VERSION_ARM64
fi

# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y erlang-base=$ERLANG_VER \
                        erlang-asn1=$ERLANG_VER erlang-crypto=$ERLANG_VER erlang-eldap=$ERLANG_VER erlang-ftp=$ERLANG_VER erlang-inets=$ERLANG_VER \
                        erlang-mnesia=$ERLANG_VER erlang-os-mon=$ERLANG_VER erlang-parsetools=$ERLANG_VER erlang-public-key=$ERLANG_VER \
                        erlang-runtime-tools=$ERLANG_VER erlang-snmp=$ERLANG_VER erlang-ssl=$ERLANG_VER \
                        erlang-syntax-tools=$ERLANG_VER erlang-tftp=$ERLANG_VER erlang-tools=$ERLANG_VER erlang-xmerl=$ERLANG_VER 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo erlang install error, exiting
    exit $?
fi

# apt-get install has a habit to write "Running kernel seems to be up-to-date." to stderr. Ignore it and rely on the exit code
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --fix-missing rabbitmq-server=$RABBITMQ_VER 2>/dev/null
if [ "$?" -ne "0" ]; then
    echo rabbitmq install error, exiting
    exit $?
fi
