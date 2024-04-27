{
  // Variables to play with

  local dep_name = 'sampleaws001',  // Can be any combination of alphanumeric characters. Make it unique.
  local provider_name = 'aws',
  local subnet_availability_zone = 'us-east-1a', // Used by AWS, not used by Openstack
  local cassandra_node_flavor = 'aws.c7g.64', // last number is the number of cores in Cassandra nodes
  local architecture = 'arm64', // amd64 or arm64 
  local cassandra_total_nodes = 4, // Cassandra cluster size - 4,8,16
  local daemon_total_instances = cassandra_total_nodes, // If tasks are CPU-intensive (Python calc), make it equal to cassandra_total_nodes, otherwise cassandra_total_nodes/2
  local DEFAULT_DAEMON_THREAD_POOL_SIZE = '24', // max daemon_cores*1.5
  local DEFAULT_DAEMON_DB_WRITERS = '16', // Depends on cassandra latency, reasonable values are 5-20

  // It's unlikely that you need to change anything below this line

  // Network
  // This is what external network is called for this cloud provider (used by Openstack)
  local external_gateway_network_name = 'ext-network-not-needed-for-aws',

  local vpc_cidr = '10.5.0.0/16', // AWS only
  local private_subnet_cidr = '10.5.0.0/24',
  local public_subnet_cidr = '10.5.1.0/24', // AWS only
  local private_subnet_allocation_pool = 'start=10.5.0.240,end=10.5.0.254',  // We use fixed ip addresses in the .0.2-.0.239 range, the rest is potentially available
  local bastion_subnet_type = if provider_name == 'aws' then 'public' else 'private',

  // Internal IPs
  local internal_bastion_ip = if provider_name == 'aws' then '10.5.1.10' else '10.5.0.10', // In AWS, bastion is in the public subnet 10.5.1.0/24
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
  local instance_availability_zone = 'not-used-borrowed-from-subnet', // Used by Openstack, AWS borrows availability zone from the subnet
  local instance_image_name = 
    if architecture == 'arm64' then 'ami-064b469793e32e5d2' // ubuntu/images/hvm-ssd/ubuntu-lunar-23.04-arm64-server-20230904
    else if architecture == 'amd64' then 'ami-0d8583a0d8d6dd14f' //ubuntu/images/hvm-ssd/ubuntu-lunar-23.04-amd64-server-20230714
    else 'unknown-architecture-unknown-image',
  
  local instance_flavor_rabbitmq = 
    if architecture == 'arm64' then 'c7g.medium'
    else if architecture == 'amd64' then 't2.micro'
    else 'unknown-architecture-unknown-rabbitmq-flavor',

  local instance_flavor_prometheus = 
    if architecture == 'arm64' then 'c7g.medium'
    else if architecture == 'amd64' then 't2.micro'
    else 'unknown-architecture-unknown-prometheus-flavor',

  // Something modest, but capable of serving as NFS server, Webapi, UI and log collector
  local instance_flavor_bastion =
    if architecture == 'arm64' then 'c7g.large'
    else if architecture == 'amd64' then 't2.medium'
    else 'unknown-architecture-unknown-prometheus-flavor',

  // Fast/big everything: CPU, network, disk, RAM. Preferably local disk, preferably bare metal 
  local instance_flavor_cassandra = getFromMap({
      'aws.c6a.16': 'c6a.4xlarge',
      'aws.c6a.32': 'c5ad.8xlarge', // 'c5ad.8xlarge' 2x600, c5ad.16xlarge' 2x1200
      'aws.c6a.64': 'c6ad.16xlarge',

      'aws.c7g.16': 'c7g.4xlarge',
      'aws.c7g.32': 'c7gd.8xlarge', // 1x900
      'aws.c7g.64': 'c7gd.16xlarge', // 2x1900
  }, cassandra_node_flavor),

  // Fast/big CPU, network, RAM. Disk optional.
  local instance_flavor_daemon = getFromMap({
      'aws.c6a.16': 'c6a.xlarge',
      'aws.c6a.32': 'c6a.2xlarge',
      'aws.c6a.64': 'c6a.4xlarge',

      'aws.c7g.16': 'c7g.xlarge',
      'aws.c7g.32': 'c7g.2xlarge',
      'aws.c7g.64': 'c7g.4xlarge',
  }, cassandra_node_flavor),

  // Whatever lsblk says
  local cassandra_nvme_regex = 
    if instance_flavor_cassandra == "c5ad.8xlarge" then "nvme[0-9]n[0-9] 558.8G"
    else if instance_flavor_cassandra == "c7gd.8xlarge" then "nvme[0-9]n[0-9] 1.7T"
    else if instance_flavor_cassandra == "c7gd.16xlarge" then "nvme[0-9]n[0-9] 1.7T"
    else "unknown-nvme-mask",

  // Volumes
  local volume_availability_zone = subnet_availability_zone, // Keep it simple

  // Something modest to store in/out data and cfg
  local volume_type = 'gp2',
  
  // Prometheus and exporters versions
  local prometheus_node_exporter_version = '1.6.0',
  local prometheus_server_version = '2.45.0',
  local jmx_exporter_version = '0.20.0',

  // Used by Prometheus "\\'localhost:9100\\',\\'10.5.0.10:9100\\',\\'10.5.0.5:9100\\',\\'10.5.0.11:9100\\'...",
  local prometheus_targets = std.format("\\'localhost:9100\\',\\'%s:9100\\',\\'%s:9100\\',", [internal_bastion_ip, rabbitmq_ip]) + // Prometheus node exporter
                             "\\'" + std.join(":9100\\',\\'", cassandra_ips) + ":9100\\'," + // Prometheus node exporter
                             "\\'" + std.join(":7070\\',\\'", cassandra_ips) + ":7070\\'," + // JMX exporter
                             "\\'" + std.join(":9100\\',\\'", daemon_ips) + ":9100\\'",      // Prometheus node exporter

  deploy_provider_name: provider_name,

  // Full list of env variables expected by capideploy working with this project
  env_variables_used: [
    // Used in this config
    'CAPIDEPLOY_SSH_USER',
    'CAPIDEPLOY_SSH_PRIVATE_KEY_PATH',
    'CAPIDEPLOY_SSH_PRIVATE_KEY_PASS',
    'CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME',
    'CAPIDEPLOY_CAPILLARIES_ROOT_DIR',

    'CAPIDEPLOY_RABBITMQ_ADMIN_NAME',
    'CAPIDEPLOY_RABBITMQ_ADMIN_PASS',
    'CAPIDEPLOY_RABBITMQ_USER_NAME',
    'CAPIDEPLOY_RABBITMQ_USER_PASS',

    // Copied to daemon machines for s3 access
    'CAPIDEPLOY_IAM_AWS_ACCESS_KEY_ID',
    'CAPIDEPLOY_IAM_AWS_SECRET_ACCESS_KEY',
    'CAPIDEPLOY_IAM_AWS_DEFAULT_REGION',
  ],
  ssh_config: {
    external_ip_address: '',
    port: 22,
    user: '{CAPIDEPLOY_SSH_USER}',
    private_key_path: '{CAPIDEPLOY_SSH_PRIVATE_KEY_PATH}',
    private_key_password: '{CAPIDEPLOY_SSH_PRIVATE_KEY_PASS}',
  },
  timeouts: {
  },

  artifacts: {
    env: {
      DIR_CAPILLARIES_ROOT: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}',
      DIR_BUILD_LINUX_AMD64: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/linux/amd64',
      DIR_BUILD_LINUX_ARM64: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/linux/arm64',
      DIR_SRC_CA: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/test/ca',
      DIR_BUILD_CA: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/ca',
      DIR_PKG_EXE: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/ca/pkg/exe',
      DIR_CODE_PARQUET: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/test/code/parquet',
    },
    cmd: [
      'sh/local/build_binaries.sh',
      'sh/local/build_webui.sh',
    ],
  },

  network: {
    name: dep_name + '_network',
    cidr: vpc_cidr,
    private_subnet: {
      name: dep_name + '_private_subnet',
      cidr: private_subnet_cidr,
      availability_zone: subnet_availability_zone,
      allocation_pool: private_subnet_allocation_pool,
    },
    public_subnet: {
      name: dep_name + '_public_subnet',
      cidr: public_subnet_cidr,
      availability_zone: subnet_availability_zone,
      nat_gateway_name: dep_name + '_natgw',
    },
    router: { // aka AWS internet gateway
      name: dep_name + '_router',
      external_gateway_network_name: external_gateway_network_name,
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
          desc: 'Capillaries webapi',
          protocol: 'tcp',
          ethertype: 'IPv4',
          remote_ip: '0.0.0.0/0',
          port: 6543,
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
  file_groups_up: {
    // daemon/webapi/toolbelt will use it to access https files
    up_ca: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/ca/all.tgz',
      dst: '/home/' + $.ssh_config.user + '/bin/ca', // $ENV_CONFIG_FILE ca_path settings points here
      dir_permissions: 744,
      file_permissions: 644,
      after: {
        env: {
          CAPI_BINARY_ROOT: '/home/' + $.ssh_config.user + '/bin'
        },
        cmd: [
          'sh/capiscripts/unpack_ca.sh',
        ],
      },
    },
    up_capiparquet_binary: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/linux/arm64/capiparquet.gz',
      dst: '/home/' + $.ssh_config.user + '/bin',
      dir_permissions: 744,
      file_permissions: 644,
      after: {
        env: {
          CAPI_BINARY: '/home/' + $.ssh_config.user + '/bin/capiparquet',
        },
        cmd: [
          'sh/capiscripts/unpack_capi_binary.sh',
        ],
      },
    },
    up_daemon_binary: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/linux/' + architecture + '/capidaemon.gz',
      dst: '/home/' + $.ssh_config.user + '/bin',
      dir_permissions: 744,
      file_permissions: 644,
      after: {
        env: {
          CAPI_BINARY: '/home/' + $.ssh_config.user + '/bin/capidaemon',
        },
        cmd: [
          'sh/capiscripts/unpack_capi_binary.sh',
        ],
      },
    },
    up_daemon_env_config: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/ca/pkg/exe/daemon/capidaemon.json',
      dst: '/home/' + $.ssh_config.user + '/bin',
      dir_permissions: 744,
      file_permissions: 644,
      after: {},
    },
    up_toolbelt_binary: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/linux/' + architecture + '/capitoolbelt.gz',
      dst: '/home/' + $.ssh_config.user + '/bin',
      dir_permissions: 744,
      file_permissions: 644,
      after: {
        env: {
          CAPI_BINARY: '/home/' + $.ssh_config.user + '/bin/capitoolbelt',
        },
        cmd: [
          'sh/capiscripts/unpack_capi_binary.sh',
        ],
      },
    },
    up_toolbelt_env_config: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/ca/pkg/exe/toolbelt/capitoolbelt.json',
      dst: '/home/' + $.ssh_config.user + '/bin',
      dir_permissions: 744,
      file_permissions: 644,
      after: {},
    },
    up_ui: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/ui/build/all.tgz',
      dst: '/home/' + $.ssh_config.user + '/ui',
      dir_permissions: 755,
      file_permissions: 644,
      after: {
        env: {
          UI_ROOT: '/home/' + $.ssh_config.user + '/ui',
        },
        cmd: [
          'sh/capiscripts/unpack_ui.sh',
        ],
      },

    },
    up_webapi_binary: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/linux/' + architecture + '/capiwebapi.gz',
      dst: '/home/' + $.ssh_config.user + '/bin',
      dir_permissions: 744,
      file_permissions: 644,
      after: {
        env: {
          CAPI_BINARY: '/home/' + $.ssh_config.user + '/bin/capiwebapi',
        },
        cmd: [
          'sh/capiscripts/unpack_capi_binary.sh',
        ],
      },
    },
    up_webapi_env_config: {
      src: '{CAPIDEPLOY_CAPILLARIES_ROOT_DIR}/build/ca/pkg/exe/webapi/capiwebapi.json',
      dst: '/home/' + $.ssh_config.user + '/bin',
      dir_permissions: 744,
      file_permissions: 644,
      after: {},
    },
  },
  file_groups_down: {
    down_capi_logs: {
      src: '/var/log/capidaemon/',
      dst: './tmp/capi_logs',
    },
  },

  // Only alphanumeric characters allowed in instance names! No underscores, no dashes, no dots, no spaces - nada.

  local bastion_instance = {
    bastion: {
      purpose: 'bastion',
      host_name: dep_name + '-bastion',
      security_group: 'bastion',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: internal_bastion_ip,
      uses_ssh_config_external_ip_address: true,
      flavor: instance_flavor_bastion,
      image: instance_image_name,
      availability_zone: instance_availability_zone,
      subnet_type: bastion_subnet_type,
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
          AMQP_URL: 'amqp://{CAPIDEPLOY_RABBITMQ_USER_NAME}:{CAPIDEPLOY_RABBITMQ_USER_PASS}@' + rabbitmq_ip + '/',
          CASSANDRA_HOSTS: cassandra_hosts,
          PROMETHEUS_IP: prometheus_ip,
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          RABBITMQ_IP: rabbitmq_ip,
          SSH_USER: $.ssh_config.user,
          NETWORK_CIDR: $.network.cidr,
          EXTERNAL_IP_ADDRESS: '{EXTERNAL_IP_ADDRESS}',  // internal: capideploy populates it from ssh_config.external_ip_address after loading project file; used by webui and webapi config.sh
          WEBAPI_PORT: '6543',
        },
        cmd: {
          install: [
            'sh/common/replace_nameserver.sh',
            'sh/common/increase_ssh_connection_limit.sh',
            'sh/prometheus/install_node_exporter.sh',
            'sh/nginx/install.sh',
          ],
          config: [
            'sh/prometheus/config_node_exporter.sh',
            'sh/rsyslog/config_capidaemon_log_receiver.sh',
            'sh/logrotate/config_capidaemon_logrotate_bastion.sh',
            'sh/toolbelt/config.sh',
            'sh/webapi/config.sh',
            'sh/ui/config.sh',
            'sh/nginx/config_ui.sh',
            'sh/nginx/config_prometheus_reverse_proxy.sh',
            'sh/nginx/config_rabbitmq_reverse_proxy.sh',
          ],
          start: [
            'sh/webapi/start.sh',
            'sh/nginx/start.sh',
          ],
          stop: [
            'sh/webapi/stop.sh',
            'sh/nginx/stop.sh',
          ],
        },
      },
      applicable_file_groups: [
        'up_webapi_binary',
        'up_webapi_env_config',
        'up_toolbelt_binary',
        'up_toolbelt_env_config',
        'up_capiparquet_binary',
        'up_ca',
        'up_ui',
        'down_capi_logs',
      ],
    },
  },

  local rabbitmq_instance = {
    rabbitmq: {
      purpose: 'rabbitmq',
      host_name: dep_name + '-rabbitmq',
      security_group: 'internal',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: rabbitmq_ip,
      flavor: instance_flavor_rabbitmq,
      image: instance_image_name,
      availability_zone: instance_availability_zone,
      subnet_type: 'private',
      service: {
        env: {
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          RABBITMQ_ADMIN_NAME: '{CAPIDEPLOY_RABBITMQ_ADMIN_NAME}',
          RABBITMQ_ADMIN_PASS: '{CAPIDEPLOY_RABBITMQ_ADMIN_PASS}',
          RABBITMQ_USER_NAME: '{CAPIDEPLOY_RABBITMQ_USER_NAME}',
          RABBITMQ_USER_PASS: '{CAPIDEPLOY_RABBITMQ_USER_PASS}',
        },
        cmd: {
          install: [
            'sh/common/replace_nameserver.sh',
            'sh/prometheus/install_node_exporter.sh',
            'sh/rabbitmq/install.sh',
          ],
          config: [
            'sh/prometheus/config_node_exporter.sh',
            'sh/rabbitmq/config.sh',
          ],
          start: [
            'sh/rabbitmq/start.sh',
          ],
          stop: [
            'sh/rabbitmq/stop.sh',
          ],
        },
      },
    },
  },

  local prometheus_instance = {
    prometheus: {
      purpose: 'prometheus',
      host_name: dep_name + '-prometheus',
      security_group: 'internal',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: prometheus_ip,
      flavor: instance_flavor_prometheus,
      image: instance_image_name,
      availability_zone: instance_availability_zone,
      subnet_type: 'private',
      service: {
        env: {
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          PROMETHEUS_TARGETS: prometheus_targets,
          PROMETHEUS_VERSION: prometheus_server_version,
        },
        cmd: {
          install: [
            'sh/common/replace_nameserver.sh',
            'sh/prometheus/install_server.sh',
            'sh/prometheus/install_node_exporter.sh',
          ],
          config: [
            'sh/prometheus/config_server.sh',
            'sh/prometheus/config_node_exporter.sh',
          ],
          start: [
            'sh/prometheus/start_server.sh',
          ],
          stop: [
            'sh/prometheus/stop_server.sh',
          ],
        },
      },
    },
  },

  local cass_instances = {
    [e.nickname]: {
      purpose: 'cassandra',
      host_name: e.host_name,
      security_group: 'internal',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: e.ip_address,
      flavor: instance_flavor_cassandra,
      image: instance_image_name,
      availability_zone: instance_availability_zone,
      subnet_type: 'private',
      service: {
        env: {
          CASSANDRA_IP: e.ip_address,
          CASSANDRA_SEEDS: cassandra_seeds,
          INITIAL_TOKEN: e.token,
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          JMX_EXPORTER_VERSION: jmx_exporter_version,
          NVME_REGEX: cassandra_nvme_regex,
        },
        cmd: {
          install: [
            'sh/common/replace_nameserver.sh',
            'sh/prometheus/install_node_exporter.sh',
            'sh/cassandra/install.sh',
            'sh/common/attach_nvme.sh', // must run after Cassandra install
          ],
          config: [
            'sh/prometheus/config_node_exporter.sh',
            'sh/cassandra/config.sh',
          ],
          start: [
            'sh/cassandra/start.sh',
          ],
          stop: [
            'sh/cassandra/stop.sh',
          ],
        },
      },
    }
    for e in std.mapWithIndex(function(i, v) {
      nickname: std.format('cass%03d', i + 1),
      host_name: dep_name + '-' + self.nickname,
      token: cassandra_tokens[i],
      ip_address: v,
    }, cassandra_ips)
  },

  local daemon_instances = {
    [e.nickname]: {
      purpose: 'daemon',
      host_name: e.host_name,
      security_group: 'internal',
      root_key_name: '{CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME}',
      ip_address: e.ip_address,
      flavor: instance_flavor_daemon,
      image: instance_image_name,
      availability_zone: instance_availability_zone,
      subnet_type: 'private',
      service: {
        env: {
          AMQP_URL: 'amqp://{CAPIDEPLOY_RABBITMQ_USER_NAME}:{CAPIDEPLOY_RABBITMQ_USER_PASS}@' + rabbitmq_ip + '/',
          CASSANDRA_HOSTS: cassandra_hosts,
          DAEMON_THREAD_POOL_SIZE: DEFAULT_DAEMON_THREAD_POOL_SIZE,
          DAEMON_DB_WRITERS: DEFAULT_DAEMON_DB_WRITERS,
          INTERNAL_BASTION_IP: internal_bastion_ip,
          PROMETHEUS_NODE_EXPORTER_VERSION: prometheus_node_exporter_version,
          SSH_USER: $.ssh_config.user,
        },
        cmd: {
          install: [
            'sh/common/replace_nameserver.sh',
            "sh/daemon/install.sh",
            'sh/prometheus/install_node_exporter.sh',
          ],
          config: [
            'sh/logrotate/config_capidaemon_logrotate_daemon.sh',
            'sh/prometheus/config_node_exporter.sh',
            'sh/daemon/config.sh',
            'sh/rsyslog/config_capidaemon_log_sender.sh', // This should go after daemon/config.sh, otherwise rsyslog sender does not pick up /var/log/capidaemon/capidaemon.log
          ],
          start: [
            'sh/daemon/start.sh',
            'sh/rsyslog/restart.sh', // It's stupid, but on AWS machines it's required, otherwise capidaemon.log is notpicked up whenit appears.
          ],
          stop: [
            'sh/daemon/stop.sh',
          ],
        },
      },
      applicable_file_groups: [
        'up_daemon_binary',
        'up_daemon_env_config',
        'up_ca',
      ],
    }
    for e in std.mapWithIndex(function(i, v) {
      nickname: std.format('daemon%03d', i + 1),
      host_name: dep_name + '-' + self.nickname,
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

