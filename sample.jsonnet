{
  // Variables to play with

  local provider_name = 'aws',
  local dep_name = 'sampleaws001',  // Can be any combination of alphanumeric characters. Make it unique.
  local subnet_availability_zone = 'us-east-1a', // AWS-specific
    
  // 1. aws or azure
  // 2. amd64 or arm64
  // 3. Flavor family
  // 4. Number of cores in Cassandra nodes. Daemon cores are 4 times less.
  // Check out instance_flavor below for full list of available setups or add your own
  local deployment_flavor_power = provider_name + '.arm64.c7g.16',

   // Cassandra cluster size - 4,8,16
  local cassandra_total_nodes = 4,

  // You probably will not change anything below this line

  // max: daemon_cores*1.5 (which is the same as cassandra cores / 4 * 1.5)
  local DEFAULT_DAEMON_THREAD_POOL_SIZE = std.toString(std.parseInt(std.split(deployment_flavor_power,".")[3]) / 4 * 1.5), 

  // Depends on cassandra latency, reasonable values are 5-20. Let it be daemon cores
  local DEFAULT_DAEMON_DB_WRITERS = std.toString(std.parseInt(std.split(deployment_flavor_power,".")[3]) / 4), 

  // If tasks are CPU-intensive (Python calc), make it equal to cassandra_total_nodes, otherwise cassandra_total_nodes/2 may be enough
  local daemon_total_instances = cassandra_total_nodes, 

  // It's very unlikely that you need to change anything below this line

  local architecture =  std.split(deployment_flavor_power,".")[1], // amd64 or arm64 
  local os_arch = 'linux/' + architecture,

  // Network
  local vpc_cidr = '10.5.0.0/16', // AWS only
  local private_subnet_cidr = '10.5.0.0/24',
  local public_subnet_cidr = '10.5.1.0/24', // AWS only

  // Internal IPs
  local internal_bastion_ip = '10.5.1.10',
  local prometheus_ip = '10.5.0.4',
  local rabbitmq_ip = '10.5.0.5',
  local daemon_ips = 
    if daemon_total_instances == 2 then ['10.5.0.101', '10.5.0.102']
    else if daemon_total_instances == 4 then ['10.5.0.101', '10.5.0.102', '10.5.0.103', '10.5.0.104']
    else if daemon_total_instances == 8 then ['10.5.0.101', '10.5.0.102', '10.5.0.103', '10.5.0.104', '10.5.0.105', '10.5.0.106', '10.5.0.107', '10.5.0.108']
    else if daemon_total_instances == 16 then ['10.5.0.101', '10.5.0.102', '10.5.0.103', '10.5.0.104', '10.5.0.105', '10.5.0.106', '10.5.0.107', '10.5.0.108', '10.5.0.109', '10.5.0.110', '10.5.0.111', '10.5.0.112', '10.5.0.113', '10.5.0.114', '10.5.0.115', '10.5.0.116']
    else [],
  local cassandra_ips = 
    if cassandra_total_nodes == 4 then ['10.5.0.11', '10.5.0.12', '10.5.0.13', '10.5.0.14']
    else if cassandra_total_nodes == 8 then ['10.5.0.11', '10.5.0.12', '10.5.0.13', '10.5.0.14', '10.5.0.15', '10.5.0.16', '10.5.0.17', '10.5.0.18']
    else if cassandra_total_nodes == 16 then ['10.5.0.11', '10.5.0.12', '10.5.0.13', '10.5.0.14', '10.5.0.15', '10.5.0.16', '10.5.0.17', '10.5.0.18', '10.5.0.19', '10.5.0.20', '10.5.0.21', '10.5.0.22', '10.5.0.23', '10.5.0.24', '10.5.0.25', '10.5.0.26']
    else [],

  // Cassandra-specific
  local cassandra_tokens = // Initial tokens to speedup bootstrapping
    if cassandra_total_nodes == 4 then ['-9223372036854775808', '-4611686018427387904', '0', '4611686018427387904']
    else if cassandra_total_nodes == 8 then ['-9223372036854775808', '-6917529027641081856', '-4611686018427387904', '-2305843009213693952', '0', '2305843009213693952', '4611686018427387904', '6917529027641081856']
    else if cassandra_total_nodes == 16 then ['-9223372036854775808','-8070450532247928832','-6917529027641081856','-5764607523034234880','-4611686018427387904','-3458764513820540928','-2305843009213693952','-1152921504606846976','0','1152921504606846976','2305843009213693952','3458764513820540928','4611686018427387904','5764607523034234880','6917529027641081856','8070450532247928832']
    else [],
  local cassandra_seeds = std.join(',', cassandra_ips),  // Used by cassandra nodes, all are seeds to avoid bootstrapping
  local cassandra_hosts = "'[\"" + std.join('","', cassandra_ips) + "\"]'",  // Used by daemons "'[\"10.5.0.11\",\"10.5.0.12\",\"10.5.0.13\",\"10.5.0.14\",\"10.5.0.15\",\"10.5.0.16\",\"10.5.0.17\",\"10.5.0.18\"]'",
  
  // Instances
  local instance_image_id = 
    if architecture == 'arm64' then 'ami-09b2701695676705d'// ubuntu/images/hvm-ssd/ubuntu-lunar-23.04-arm64-server-20240117 // 'ami-064b469793e32e5d2' ubuntu/images/hvm-ssd/ubuntu-lunar-23.04-arm64-server-20230904
    else if architecture == 'amd64' then 'ami-0d8583a0d8d6dd14f' //ubuntu/images/hvm-ssd/ubuntu-lunar-23.04-amd64-server-20230714
    else 'unknown-architecture-unknown-image',

  local instance_flavor = getFromMap({
    'aws.amd64.c6a.16': {cassandra:'c6a.4xlarge',   cass_nvme_regex:'unknown-nvme-regex',     daemon: 'c6a.xlarge',  rabbitmq: 't2.micro',   prometheus: 't2.micro',   bastion: 't2.micro' },
    'aws.amd64.c6a.32': {cassandra:'c5ad.8xlarge',  cass_nvme_regex:'nvme[0-9]n[0-9] 558.8G', daemon: 'c6a.2xlarge', rabbitmq: 't2.micro',   prometheus: 't2.micro',   bastion: 't2.micro' },
    'aws.amd64.c6a.64': {cassandra:'c6ad.16xlarge', cass_nvme_regex:'unknown-nvme-regex',     daemon: 'c6a.4xlarge', rabbitmq: 't2.micro',   prometheus: 't2.micro',   bastion: 't2.micro' },
    'aws.arm64.c7g.16': {cassandra:'c7gd.4xlarge',  cass_nvme_regex:'nvme[0-9]n[0-9] 884.8G', daemon: 'c7g.xlarge',  rabbitmq: 'c7g.medium', prometheus: 'c7g.medium', bastion: 'c7g.large'},
    'aws.arm64.c7g.32': {cassandra:'c7gd.8xlarge',  cass_nvme_regex:'nvme[0-9]n[0-9] 1.7T',   daemon: 'c7g.2xlarge', rabbitmq: 'c7g.medium', prometheus: 'c7g.medium', bastion: 'c7g.large'},
    'aws.arm64.c7g.64': {cassandra:'c7gd.16xlarge', cass_nvme_regex:'nvme[0-9]n[0-9] 1.7T',   daemon: 'c7g.4xlarge', rabbitmq: 'c7g.medium', prometheus: 'c7g.medium', bastion: 'c7g.large'}
  }, deployment_flavor_power),

  // Volumes
  local volume_availability_zone = subnet_availability_zone, // Keep it simple

  // Something modest to store in/out data and cfg
  local volume_type = 'gp2',
  
  // Prometheus and exporters versions
  local prometheus_node_exporter_version = '1.6.0',
  local prometheus_server_version = '2.45.0',
  local jmx_exporter_version = '0.20.0',

  // Used by Prometheus "\\'localhost:9100\\',\\'10.5.1.10:9100\\',\\'10.5.0.5:9100\\',\\'10.5.0.11:9100\\'...",
  local prometheus_targets = std.format("\\'localhost:9100\\',\\'%s:9100\\',\\'%s:9100\\',", [internal_bastion_ip, rabbitmq_ip]) + // Prometheus node exporter
                             "\\'" + std.join(":9100\\',\\'", cassandra_ips) + ":9100\\'," + // Prometheus node exporter
                             "\\'" + std.join(":7070\\',\\'", cassandra_ips) + ":7070\\'," + // JMX exporter
                             "\\'" + std.join(":9100\\',\\'", daemon_ips) + ":9100\\'",      // Prometheus node exporter

  deployment_name: dep_name,
  deploy_provider_name: provider_name,

  ssh_config: {
    bastion_external_ip_address_name: dep_name +  '_bastion_external_ip_name',
    // external_ip_address: '',
    port: 22,
    user: '{CAPIDEPLOY_SSH_USER}',
    private_key_path: '{CAPIDEPLOY_SSH_PRIVATE_KEY_PATH}',
  },
  timeouts: {
  },

  network: {
    name: dep_name + '_network',
    cidr: vpc_cidr,
    private_subnet: {
      name: dep_name + '_private_subnet',
      route_table_to_nat_gateway_name: dep_name + '_private_subnet_rt_to_natgw',
      cidr: private_subnet_cidr,
      availability_zone: subnet_availability_zone,
    },
    public_subnet: {
      name: dep_name + '_public_subnet',
      cidr: public_subnet_cidr,
      availability_zone: subnet_availability_zone,
      nat_gateway_name: dep_name + '_natgw',
      nat_gateway_external_ip_address_name: dep_name + '_natgw_external_ip_name',
    },
    router: { // aka AWS internet gateway
      name: dep_name + '_router',
    },
  },
  security_groups: {
    bastion: {
      name: dep_name + '_bastion_security_group',
      rules: [
        {
          desc: 'SSH',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: '0.0.0.0/0',
          port: 22,
          direction: 'ingress',
        },
        {
          desc: 'Prometheus UI reverse proxy',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: '0.0.0.0/0',
          port: 9090,
          direction: 'ingress',
        },
        {
          desc: 'Prometheus node exporter',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 9100,
          direction: 'ingress',
        },
        {
          desc: 'RabbitMQ UI reverse proxy',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: '0.0.0.0/0',
          port: 15672,
          direction: 'ingress',
        },
        {
          desc: 'rsyslog receiver',
          protocol: 'udp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 514,
          direction: 'ingress',
        },
        {
          desc: 'Capillaries external webapi',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: '0.0.0.0/0',
          port: 6544,
          direction: 'ingress',
        },
        {
          desc: 'Capillaries UI nginx',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: '0.0.0.0/0',
          port: 80,
          direction: 'ingress',
        },
      ],
    },
    internal: {
      name: dep_name + '_internal_security_group',
      rules: [
        {
          desc: 'SSH',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 22,
          direction: 'ingress',
        },
        {
          desc: 'Prometheus UI internal',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 9090,
          direction: 'ingress',
        },
        {
          desc: 'Prometheus node exporter',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 9100,
          direction: 'ingress',
        },
        {
          desc: 'JMX exporter',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 7070,
          direction: 'ingress',
        },
        {
          desc: 'Cassandra JMX',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 7199,
          direction: 'ingress',
        },
        {
          desc: 'Cassandra cluster comm',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 7000,
          direction: 'ingress',
        },
        {
          desc: 'Cassandra API',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 9042,
          direction: 'ingress',
        },
        {
          desc: 'RabbitMQ API',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 5672,
          direction: 'ingress',
        },
        {
          desc: 'RabbitMQ UI',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: $.network.cidr,
          port: 15672,
          direction: 'ingress',
        },
      ],
    },
  },

  // Only alphanumeric characters allowed in instance names! No underscores, no dashes, no dots, no spaces - nada.

  local bastion_instance = {
    bastion: {
      purpose: 'CAPIDEPLOY.INTERNAL.PURPOSE_BASTION',
      inst_name: dep_name + '-bastion',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: internal_bastion_ip,
      external_ip_address_name: $.ssh_config.bastion_external_ip_address_name,
      flavor: instance_flavor.bastion,
      image_id: instance_image_id,
      security_group_name: $.security_groups.bastion.name,
      subnet_name: $.network.public_subnet.name,
      associated_instance_profile: '{CAPIDEPLOY_INSTANCE_PROFILE_WITH_S3_ACCESS}',
      volumes: {
        'log': {
          name: dep_name + '_log',
          availability_zone: volume_availability_zone,
          mount_point: '/mnt/capi_log',
          size: 10,
          type: volume_type,
          permissions: 777,
          owner: $.ssh_config.user,
        },
      },
      service: {
        env: {
          CAPILLARIES_RELEASE_URL: '{CAPIDEPLOY_CAPILLARIES_RELEASE_URL}',
          OS_ARCH: os_arch,
          S3_IAM_USER_AWS_ACCESS_KEY_ID: '{CAPIDEPLOY_S3_IAM_USER_AWS_ACCESS_KEY_ID}',
          S3_IAM_USER_AWS_SECRET_ACCESS_KEY: '{CAPIDEPLOY_S3_IAM_USER_AWS_SECRET_ACCESS_KEY}',
          S3_AWS_DEFAULT_REGION: '{CAPIDEPLOY_S3_AWS_DEFAULT_REGION}',
          AMQP_URL: 'amqp://{CAPIDEPLOY_RABBITMQ_USER_NAME}:{CAPIDEPLOY_RABBITMQ_USER_PASS}@' + rabbitmq_ip + '/',
          CASSANDRA_HOSTS: cassandra_hosts,
          PROMETHEUS_IP: prometheus_ip,
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          RABBITMQ_IP: rabbitmq_ip,
          SSH_USER: $.ssh_config.user,
          NETWORK_CIDR: $.network.cidr,
          BASTION_ALLOWED_IPS: '{CAPIDEPLOY_BASTION_ALLOWED_IPS}',
          EXTERNAL_IP_ADDRESS: '{CAPIDEPLOY.INTERNAL.BASTION_EXTERNAL_IP_ADDRESS}',  // internal: capideploy populates it from ssh_config.external_ip_address after loading project file; used by webui and webapi config.sh
          EXTERNAL_WEBAPI_PORT: '{CAPIDEPLOY_EXTERNAL_WEBAPI_PORT}',
          INTERNAL_WEBAPI_PORT: '6543',
        },
        cmd: {
          install: [
            'scripts/common/replace_nameserver.sh',
            'scripts/common/increase_ssh_connection_limit.sh',
            'scripts/prometheus/install_node_exporter.sh',
            'scripts/nginx/install.sh',
            'scripts/ca/install.sh',
            'scripts/common/iam_aws_credentials.sh',
            'scripts/toolbelt/install.sh',
            'scripts/webapi/install.sh',
            'scripts/ui/install.sh',
          ],
          config: [
            'scripts/prometheus/config_node_exporter.sh',
            'scripts/rsyslog/config_catchall_log_receiver.sh',
            'scripts/logrotate/config_bastion.sh',
            'scripts/toolbelt/config.sh',
            'scripts/webapi/config.sh',
            'scripts/ui/config.sh',
            'scripts/nginx/config_whitelist.sh',
            'scripts/nginx/config_ui.sh',
            'scripts/nginx/config_webapi_reverse_proxy.sh',
            'scripts/nginx/config_prometheus_reverse_proxy.sh',
            'scripts/nginx/config_rabbitmq_reverse_proxy.sh',
          ],
          start: [
            'scripts/rsyslog/start.sh',
            'scripts/logrotate/start.sh',
            'scripts/webapi/start.sh',
            'scripts/nginx/start.sh',
          ],
          stop: [
            'scripts/webapi/stop.sh',
            'scripts/nginx/stop.sh',
            'scripts/logrotate/stop.sh',
            'scripts/rsyslog/stop.sh',
          ],
        },
      },
    },
  },

  local rabbitmq_instance = {
    rabbitmq: {
      purpose: 'CAPIDEPLOY.INTERNAL.PURPOSE_RABBITMQ',
      inst_name: dep_name + '-rabbitmq',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: rabbitmq_ip,
      flavor: instance_flavor.rabbitmq,
      image_id: instance_image_id,
      security_group_name: $.security_groups.internal.name,
      subnet_name: $.network.private_subnet.name,
      service: {
        env: {
          INTERNAL_BASTION_IP: internal_bastion_ip,
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          RABBITMQ_ADMIN_NAME: '{CAPIDEPLOY_RABBITMQ_ADMIN_NAME}',
          RABBITMQ_ADMIN_PASS: '{CAPIDEPLOY_RABBITMQ_ADMIN_PASS}',
          RABBITMQ_USER_NAME: '{CAPIDEPLOY_RABBITMQ_USER_NAME}',
          RABBITMQ_USER_PASS: '{CAPIDEPLOY_RABBITMQ_USER_PASS}',
        },
        cmd: {
          install: [
            'scripts/common/replace_nameserver.sh',
            'scripts/prometheus/install_node_exporter.sh',
            'scripts/rabbitmq/install.sh',
          ],
          config: [
            'scripts/prometheus/config_node_exporter.sh',
            'scripts/rabbitmq/config.sh',
            'scripts/rsyslog/config_rabbitmq_log_sender.sh',
          ],
          start: [
            'scripts/rabbitmq/start.sh',
          ],
          stop: [
            'scripts/rabbitmq/stop.sh',
          ],
        },
      },
    },
  },

  local prometheus_instance = {
    prometheus: {
      purpose: 'CAPIDEPLOY.INTERNAL.PURPOSE_PROMETHEUS',
      inst_name: dep_name + '-prometheus',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: prometheus_ip,
      flavor: instance_flavor.prometheus,
      image_id: instance_image_id,
      security_group_name: $.security_groups.internal.name,
      subnet_name: $.network.private_subnet.name,
      service: {
        env: {
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          PROMETHEUS_TARGETS: prometheus_targets,
          PROMETHEUS_VERSION: prometheus_server_version,
        },
        cmd: {
          install: [
            'scripts/common/replace_nameserver.sh',
            'scripts/prometheus/install_server.sh',
            'scripts/prometheus/install_node_exporter.sh',
          ],
          config: [
            'scripts/prometheus/config_server.sh',
            'scripts/prometheus/config_node_exporter.sh',
          ],
          start: [
            'scripts/prometheus/start_server.sh',
          ],
          stop: [
            'scripts/prometheus/stop_server.sh',
          ],
        },
      },
    },
  },

  local cass_instances = {
    [e.nickname]: {
      purpose: 'CAPIDEPLOY.INTERNAL.PURPOSE_CASSANDRA',
      inst_name: e.inst_name,
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: e.ip_address,
      flavor: instance_flavor.cassandra,
      image_id: instance_image_id,
      security_group_name: $.security_groups.internal.name,
      subnet_name: $.network.private_subnet.name,
      service: {
        env: {
          INTERNAL_BASTION_IP: internal_bastion_ip,
          CASSANDRA_IP: e.ip_address,
          CASSANDRA_SEEDS: cassandra_seeds,
          INITIAL_TOKEN: e.token,
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          JMX_EXPORTER_VERSION: jmx_exporter_version,
          NVME_REGEX: instance_flavor.cass_nvme_regex,
        },
        cmd: {
          install: [
            'scripts/common/replace_nameserver.sh',
            'scripts/prometheus/install_node_exporter.sh',
            'scripts/cassandra/install.sh',
          ],
          config: [
            'scripts/prometheus/config_node_exporter.sh',
            'scripts/cassandra/config.sh',
            'scripts/rsyslog/config_cassandra_log_sender.sh',
          ],
          start: [
            'scripts/cassandra/start.sh',
            'scripts/rsyslog/restart.sh', // It's stupid, but on AWS machines it's required, otherwise the log is not picked up when it appears.
          ],
          stop: [
            'scripts/cassandra/stop.sh',
          ],
        },
      },
    }
    for e in std.mapWithIndex(function(i, v) {
      nickname: std.format('cass%03d', i + 1),
      inst_name: dep_name + '-' + self.nickname,
      token: cassandra_tokens[i],
      ip_address: v,
    }, cassandra_ips)
  },

  local daemon_instances = {
    [e.nickname]: {
      purpose: 'CAPIDEPLOY.INTERNAL.PURPOSE_DAEMON',
      inst_name: e.inst_name,
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: e.ip_address,
      flavor: instance_flavor.daemon,
      image_id: instance_image_id,
      security_group_name: $.security_groups.internal.name,
      subnet_name: $.network.private_subnet.name,
      associated_instance_profile: '{CAPIDEPLOY_INSTANCE_PROFILE_WITH_S3_ACCESS}',
      service: {
        env: {
          INTERNAL_BASTION_IP: internal_bastion_ip,
          CAPILLARIES_RELEASE_URL: '{CAPIDEPLOY_CAPILLARIES_RELEASE_URL}',
          OS_ARCH: os_arch,
          S3_IAM_USER_AWS_ACCESS_KEY_ID: '{CAPIDEPLOY_S3_IAM_USER_AWS_ACCESS_KEY_ID}',
          S3_IAM_USER_AWS_SECRET_ACCESS_KEY: '{CAPIDEPLOY_S3_IAM_USER_AWS_SECRET_ACCESS_KEY}',
          S3_AWS_DEFAULT_REGION: '{CAPIDEPLOY_S3_AWS_DEFAULT_REGION}',
          AMQP_URL: 'amqp://{CAPIDEPLOY_RABBITMQ_USER_NAME}:{CAPIDEPLOY_RABBITMQ_USER_PASS}@' + rabbitmq_ip + '/',
          CASSANDRA_HOSTS: cassandra_hosts,
          DAEMON_THREAD_POOL_SIZE: DEFAULT_DAEMON_THREAD_POOL_SIZE,
          DAEMON_DB_WRITERS: DEFAULT_DAEMON_DB_WRITERS,
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          SSH_USER: $.ssh_config.user,
        },
        cmd: {
          install: [
            'scripts/common/replace_nameserver.sh',
            "scripts/daemon/install.sh",
            'scripts/prometheus/install_node_exporter.sh',
            'scripts/common/iam_aws_credentials.sh',
            'scripts/ca/install.sh',
            'scripts/daemon/install.sh',
          ],
          config: [
            'scripts/logrotate/config_capidaemon.sh',
            'scripts/prometheus/config_node_exporter.sh',
            'scripts/daemon/config.sh',
            'scripts/rsyslog/config_capidaemon_log_sender.sh', // This should go after daemon/config.sh, otherwise rsyslog sender does not pick up /var/log/capidaemon/capidaemon.log
          ],
          start: [
            'scripts/daemon/start.sh',
            'scripts/rsyslog/restart.sh', // It's stupid, but on AWS machines it's required, otherwise the log is not picked up when it appears.
          ],
          stop: [
            'scripts/daemon/stop.sh',
          ],
        },
      },
    }
    for e in std.mapWithIndex(function(i, v) {
      nickname: std.format('daemon%03d', i + 1),
      inst_name: dep_name + '-' + self.nickname,
      ip_address: v,
    }, daemon_ips)
  },

  instances: bastion_instance + rabbitmq_instance + prometheus_instance + cass_instances + daemon_instances,

  local getFromMap = function(m, k)
    if std.length(m[k]) > 0 then m[k] else "unknown--key-" + k,

  local getFromDoubleMap = function(m, k1, k2)
    if std.length(m[k1]) > 0 then 
      if std.length(m[k1][k2]) > 0 then m[k1][k2] else "no-key-" + k2
    else  "unknown-key-" + k1,
}

