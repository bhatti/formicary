## Executors

### Shell or Local Executor
The shell executor forks a shell process from ant work for executing commands defined under `script`. It does not
require any additional configuration, but it's recommended to use a unique user for the ant worker with proper
permissions because a user may invoke any command on the machine.

### Docker Executor

### Kubernetes Executor

The Kubernetes executor starts a main container for executing script named after job/task name and a helper container
wth `-helper` suffix for managing artifacts. A task may define dependent services that will start with `svc-` prefix.
The initial kubernetes config are defined by the ant config that are available for all jobs such as:

- namespace - namespace of Kubernetes environment
- helper_image - helper image
- bearer_token - bearer token for launching pods
- host - Kubernetes api server (optional)
- cert_file - api server cert
- key_file - api server key
- ca_file - api server ca
- service_account - array of accounts to use for pods
- image_pull_secrets - array of secrets for pulling docker images
- dns_policy such as `none`, `default`, `cluster-first`, `cluster-first-with-host-net`.
- dns_config such as `nameservers`, `options`, `searches`
- volumes - to mount on pods
- pod_security_context
- host_aliases - array of host aliases
- cap_add - array of linux capabilities to add for pods
- cap_drop - array of linux capabilities to drop for pods
- environment - environment variables
- pull_policy - image pull policy such as `never`, `always`, `if-not-present`.

```yaml
common:
  id: test-id
  messaging_provider: "REDIS_MESSAGING"
tags:
  - tag1
  - tag2
methods:
  - DOCKER
  - KUBERNETES
kubernetes:
  registry:
    registry: docker-registry-server
    username: docker-registry-user
    password: docker-registry-pass
    pull_policy: if-not-present
  host: kubernetes-host
  bearer_token: kubernetes-bearer
  cert_file: kubernetes-cert
  key_file: kubernetes-key
  ca_file: kubernetes-cafile
  namespace: default
  service_account: my-svc-account
  image_pull_secrets:
    - image-pull-secret
  dns_policy: none
  pod_security_context:
    fs_group: 100
    run_as_group: 100
    run_as_non_root: true
    run_as_user: 1000
    supplemental_groups:
      - 200
      - 300
  cap_add:
    - NET_RAW
    - CAP1
  cap_drop:
    - CAP2
```

Above configuration applies to all jobs, but a kubernetes task can define following properties for each job-definition:

- name - the name of task that is used for pod-name
- environment - environment variables to set within the container
- working_directory - for script execution
- container - main container to execute, which defines:
    - image
    - image_definition
    - volumes based on host, pvc, config_map, secret and empty
        - host mounts folder from the host path
        - pvc uses persistent volume claim defined in the kubernetes cluster
        - config_map uses config map defined in the kubernetes cluster, it defines `items` to add keys and relative path
        - secret mounts secret as a volume, it defines `items` to add keys and relative path
        - empty mounts an empty volume
    - volume_driver
    - devices - array of devices
    - bind_directory
    - cpu_limit - cpu allocation given
    - cpu_request - cpu allocation requested
    - memory_limit - memory allocated
    - memory_request - memory requested
- services - array of services
    - name - service name
    - image - service image
    - command - service command
    - entrypoint - service entrypoint
    - volumes - volumes
    - cpu_limit - cpu allocation given
    - cpu_request - cpu allocation requested
    - memory_limit - memory allocated
    - memory_request - memory requested
- affinity - affinity for specifying nodes to use for execution
- node_selector - key/value pairs for selecting node with matching tolerated tainted nodes
- node_tolerations
- pod_label - key/value pairs
- pod_annotations - key/value pairs
- network_mode
- host_network e.g.,

```yaml
name: task1
method: KUBERNETES
environment:
  AWS-KEY: Mykey
container:
  image: ubuntu:16.04
  volumes:
    host_path:
      - name: mount1
        mount_path: /shared
        host_path: /host/shared
    pvc:
      - name: mount2
        mount_path: /mnt/sh1
    config_map:
      - name: mount3
        mount_path: /mnt/sh2
        items:
          item1: val1
    secret:
      - name: mount4
        mount_path: /mnt/sh3
        items:
          item1: val1
    empty_dir:
      - name: mount4
        mount_path: /mnt/sh3
  volume_driver: voldriver
  devices:
    - devices
  bind_dir: /shared
  cpu_limit: "1"
  cpu_request: 500m
  memory_limit: 1Gi
  memory_request: 1Gi
services:
  - name: svc-name
    image: ubuntu:16.04
    command:
      - cmd1
    entrypoint:
      - /bin/bash
    volumes:
      host_path:
        - name: svc-mount1
          mount_path: /shared
          host_path: /host/shared
          read_only: false
      pvc:
        - name: svc-mount2
          mount_path: /mnt/sh1
          read_only: true
      config_map:
        - name: svc-mount3
          mount_path: /mnt/sh2
          read_only: true
          items:
            item1: val1
      secret:
        - name: svc-mount4
          mount_path: /mnt/sh3
          items:
            mysecret: file-name
      empty_dir:
        - name: svc-mount5
          mount_path: /mnt/sh3
    cpu_limit: "1"
    cpu_request: 500m
    memory_limit: 1Gi
    memory_request: 1Gi
privileged: true
affinity:
  required_during_scheduling_ignored_during_execution:
    node_selector_terms:
      - match_expressions:
          - key: datacenter
            operator: In
            values:
              - seattle
        match_fields:
          - key: key2
            operator: In
            values:
              - val2
  preferred_during_scheduling_ignored_during_execution:
    - weight: 1
      preference:
        match_expressions:
          - key: datacenter
            operator: In
            values:
              - chicago
        match_fields:
          - key: color
            operator: In
            values:
              - blue
node_selector:
  formicary: "true"
node_tolerations:
  empty: PreferNoSchedule
  myrole: NoSchedule
pod_labels:
  foo: bar
pod_annotations:
  ann1: val
network_mode: mod1
host_network: true
```

