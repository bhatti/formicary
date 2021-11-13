## Advanced Kubernetes

### Kubernetes Jobs with Volumes and Services
Following is an example job-definition that creates Kubernetes Volume, Mounts and Services:

#### Job Configuration
```yaml
job_type: kube-example1
description: Simple Kubernetes example with volume mounts, secrets and ports
max_concurrency: 1
tasks:
- task_type: kubby
  tags:
  - builder
  pod_labels:
    foor: bar
  script:
    - ls -l /myshared
    - ls -l /myempty
    - sleep 30
  method: KUBERNETES
  host_network: false
  services:
    - name: redis
      image: redis:6.2.2-alpine
      ports:
        - number: 6379
  container:
    image: ubuntu:16.04
    volumes:
      host_path:
        - name: mount1
          mount_path: /myshared
          host_path: /shared
      empty_dir:
        - name: mount2
          mount_path: /myempty
      projected:
        - name: oidc-token
          mount_path: /var/run/sigstore/cosign
          sources:
            - service_account_token:
              path: oidc-token
              expiration_seconds: 600
              audience: sigstore
```

You can store above definition in a file such as `kube-example1.yaml` and then upload to formicary using
```bash
curl -H "Content-Type: application/yaml" \
    --data-binary @kube-example1.yaml \
    $SERVER/jobs/definitions
```

Then submit your job using:

```bash
curl -H "Content-Type: application/json" \
    --data '{"job_type": "kube-example1", "org_unit": "myorg", "username": "myuser", "params": {"Platform": "Test"}}' \
    $SERVER/jobs/requests
```

Next, open dashboard on your browser to view the running jobs.

You can use `kubctl describe pod <podname>` to verify labels, volumes or services such as:
```
 Labels:       AntID=formicary-ant-id1
               foor=bar
    ...
 Volumes:
   mount1:
     Type:          HostPath (bare host directory volume)
     Path:          /shared
    ... 
 Mounts:
   /myshared from mount1 (rw)     
    ...
 Containers:
   svc-redis:
     Container ID:
     Image:          redis:6.2.2-alpine
    ...
```

You may see errors
like ` Warning  FailedScheduling  20s   default-scheduler  0/1 nodes are available: 1 persistentvolumeclaim "mount2" not found`
in `kubectl describe` output when using non-existing pvc or non-existing host mounts.

### Kubernetes Jobs with Resources
Following is an example job-definition that defines resources for Kubernetes:

#### Job Configuration
```yaml
job_type: kube-example2
description: Simple Kubernetes example with resources
max_concurrency: 1
tasks:
- task_type: kubby
  tags:
  - builder
  pod_labels:
    foor: bar
  script:
    - echo hello world
    - ls -l
    - sleep 21
  method: KUBERNETES
  container:
    image: ubuntu:16.04
    cpu_limit: "1"
    cpu_request: 500m
    memory_limit: 2Gi
    memory_request: 1Gi
    ephemeral_storage_limit: 2Gi
    ephemeral_storage_request: 1Gi
```

Above example defines cpu, memory and ephemeral storage for the container. These resource options are also available for services such as:
```yaml
services:
  - name: redis
    image: redis:6.2.2-alpine
    ports:
      - number: 123
        name: name
    cpu_limit: "1"
    cpu_request: 500m
    memory_limit: 1Gi
    memory_request: 1Gi
    ephemeral_storage_limit: 2Gi
    ephemeral_storage_request: 1Gi
```

You can upload/submit your job similar to previous example and then verify these limits using `kublectl describe`, e.g.
```
     Limits:
       cpu:                1
       ephemeral-storage:  2Gi
       memory:             2Gi
     Requests:
       cpu:                500m
       ephemeral-storage:  1Gi
       memory:             1Gi
```
