## Installation

### Prerequisites

 -   **Execution Environment:** Install Docker [https://docs.docker.com/engine/install/](https://hub.docker.com/search?type=edition&offering=community), Docker-Compose from [https://docs.docker.com/compose/install/](https://docs.docker.com/compose/install/), and Kubernetes cluster such as [https://microk8s.io/docs](https://microk8s.io/docs), [AWS EKS](https://aws.amazon.com/eks/), [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/), and [Azure Kubernetes Service (AKS)](https://azure.microsoft.com/en-us/products/kubernetes-service/).
 -   **Database:** Install [Goose](https://github.com/pressly/goose) and [GORM](https://gorm.io/) for relational database that supports [postgres](https://www.postgresql.org/), [mysql](https://www.mysql.com/), [sqlite3](https://www.sqlite.org/index.html), [mssql](https://www.microsoft.com/en-us/sql-server/sql-server-downloads), [redshift](https://aws.amazon.com/redshift/), [tidb](https://github.com/pingcap/tidb), [clickhouse](https://clickhouse.com/), [vertica](https://www.vertica.com/), [ydb](https://github.com/ydb-platform/ydb), and [duckdb](https://duckdb.org/).
 -   **Messaging:** Install Redis [https://redis.io/](https://redis.io/) or Apache pulsar [https://pulsar.apache.org](https://pulsar.apache.org).
 -   **Artifacts & Object Store:** Install Minio – [https://min.io/download](https://min.io/download).

## Launching Server

Here is an example `[docker-compose](https://github.com/bhatti/formicary/blob/main/docker-compose.yaml)` file designed to launch the queen-server, database server, messaging server, and object-store:
```yaml
version: '3.7'
services:
  redis:
    image: "redis:alpine"
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
  minio:
    image: minio/minio:RELEASE.2024-02-09T21-25-16Z
    volumes:
      - minio-data:/data
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: password
    command: server /data --console-address ":9001"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 30s
      timeout: 20s
      retries: 3
  mysql:
    image: "mysql:8"
    command: --default-authentication-plugin=mysql_native_password
    restart: always
    ports:
      - "3306:3306"
    environment:
      MYSQL_ALLOW_EMPTY_PASSWORD: "yes"
      DB_NAME: ${DB_NAME:-formicary_db}
      DB_USER: ${DB_USER:-formicary_user}
      DB_PASSWORD: ${DB_PASSWORD:-formicary_pass}
      DB_ROOT_USER: ${DB_ROOT_USER:-root}
      DB_ROOT_PASSWORD: ${DB_ROOT_PASSWORD:-rootroot}
      MYSQL_USER: ${DB_USER}
      MYSQL_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: ${DB_NAME}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:-rootroot}
    healthcheck:
      test: ["CMD", "mysqladmin" ,"ping", "-h", "localhost"]
      timeout: 20s
      retries: 10
    volumes:
      - mysql-data:/var/lib/mysql
#      - ./mysql-initdb:/docker-entrypoint-initdb.d
  formicary-server:
    image: plexobject/formicary
#    build:
#      context: .
#      dockerfile: Dockerfile
    depends_on:
      - redis
      - mysql
      - minio
    environment:
      COMMON_DEBUG: '${DEBUG:-false}'
      COMMON_REDIS_HOST: 'redis'
      COMMON_REDIS_PORT: '${COMMON_REDIS_PORT:-6379}'
      COMMON_S3_ENDPOINT: 'minio:9000'
      COMMON_S3_ACCESS_KEY_ID: 'admin'
      COMMON_S3_SECRET_ACCESS_KEY: 'password'
      COMMON_S3_REGION: '${AWS_DEFAULT_REGION:-us-west-2}'
      COMMON_S3_BUCKET: '${BUCKET:-formicary-artifacts}'
      COMMON_S3_PREFIX: '${PREFIX:-formicary}'
      COMMON_AUTH_GITHUB_CLIENT_ID: '${COMMON_AUTH_GITHUB_CLIENT_ID}'
      COMMON_AUTH_GITHUB_CLIENT_SECRET: '${COMMON_AUTH_GITHUB_CLIENT_SECRET}'
      COMMON_AUTH_GOOGLE_CLIENT_ID: '${COMMON_AUTH_GOOGLE_CLIENT_ID}'
      COMMON_AUTH_GOOGLE_CLIENT_SECRET: '${COMMON_AUTH_GOOGLE_CLIENT_SECRET}'
      CONFIG_FILE: ${CONFIG_FILE:-/config/formicary-queen.yaml}
      COMMON_HTTP_PORT: ${HTTP_PORT:-7777}
      DB_USER: ${DB_USER:-formicary_user}
      DB_PASSWORD: ${DB_PASSWORD:-formicary_pass}
      POSTGRES_USER: ${DB_USER:-formicary_user}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-formicary_pass}
      DB_HOST: 'mysql'
      DB_TYPE: "mysql"
      DB_DATA_SOURCE: "${DB_USER:-formicary_user}:${DB_PASSWORD:-formicary_pass}@tcp(mysql:3306)/${DB_NAME:-formicary_db}?charset=utf8mb4&parseTime=true&loc=Local"
    ports:
      - 7777:7777
    volumes:
      - ./config:/config
    entrypoint: ["/bin/sh", "-c", "/migrations/mysql_setup_db.sh migrate-only && exec /formicary --config=/config/formicary-queen.yaml --id=formicary-server-id1"]
volumes:
  minio-data:
  redis-data:
  mysql-data:
  mysql-initdb:
```

You can then define the server configuration file as follows:
```yaml
id: queen-server-id
subscription\_quota\_enabled: false
common:
messaging\_provider: REDIS\_MESSAGING
external\_base\_url: https://public-website
auth:
enabled: false
secure: true
jwt\_secret: secret-key
```

**Note:** The configuration above supports OAuth 2.0 based authentication and allows enabling of the allocation of computing resource quotas per user. Furthermore, it supports setting up notifications through email and Slack.

You can then launch the server as follows:
```yaml
docker-compose up
```

Once, the [Formicary](https://github.com/bhatti/formicary) system starts up, you can use dashboard UI or API for managing jobs at the specified host and port.

### Launching Ant Worker(s)

Here is an example `[docker-compose](https://github.com/bhatti/formicary/blob/main/ant-docker-compose.yaml)` file designed to launch the ant-worker:

```yaml
version: '3.7'
services:
  formicary-ant:
    image: plexobject/formicary
    network_mode: "host"
#    build:
#      context: .
#      dockerfile: Dockerfile
    environment:
      COMMON_DEBUG: '${DEBUG:-false}'
      COMMON_REDIS_HOST: '${QUEEN_SERVER:-192.168.1.102}'
      COMMON_REDIS_PORT: '${COMMON_REDIS_PORT:-6379}'
      COMMON_S3_ENDPOINT: '${QUEEN_SERVER:-192.168.1.102}:9000'
      COMMON_S3_ACCESS_KEY_ID: 'admin'
      COMMON_S3_SECRET_ACCESS_KEY: 'password'
      COMMON_S3_REGION: '${AWS_DEFAULT_REGION:-us-west-2}'
      COMMON_S3_BUCKET: '${BUCKET:-formicary-artifacts}'
      COMMON_S3_PREFIX: '${PREFIX:-formicary}'
      COMMON_HTTP_PORT: ${HTTP_PORT:-5555}
      CONFIG_FILE: ${CONFIG_FILE:-/config/formicary-ant.yaml}
    volumes:
      - ./config:/config
      - ./.kube:/home/formicary-user/.kube
    entrypoint: ["/bin/sh", "-c", "/formicary ant --config=/config/formicary-ant.yaml --id=formicary-ant-id1 --tags \"builder pulsar redis kotlin aws-lambda\""]
```
Above `[docker-compose](https://github.com/bhatti/formicary/blob/main/ant-docker-compose.yaml)` file mounts a kubernetes config file that you can generate using `microk8s.config` such as:
```yaml
apiVersion: v1
clusters:
- cluster:
  certificate-authority-data: LS..
  server: https://192.168.1.120:16443
  name: microk8s-cluster
  contexts:
- context:
  cluster: microk8s-cluster
  user: admin
  name: microk8s
  current-context: microk8s
  kind: Config
  preferences: {}
  users:
- name: admin
  user:
  token: V..
```

Above kubernetes configuration assumes that you are running your kubernetes cluster at `192.168.1.120` and you can change it accordingly. You can then launch the worker as follows:
```yaml
docker-compose -f ant-docker-compose.yaml up
```


#### Apache Kafka (Optional)
If you choose to use Apache Kafka as messaging middleware, you can start t as follows:
 -  zookeeper-server-start zookeeper.properties
 -  kafka-server-start server.properties

#### Apache Pulsar (Optional)
If you choose to use Apache Pulsar as messaging middleware, you can start t as follows:
 - bin/pulsar standalone

*Note*: You will need to change configuration to provide the messaging provider to `messaging_provider: REDIS_MESSAGING`, `messaging_provider: PULSAR_MESSAGING` or `messaging_provider: KAFKA_MESSAGING`

### Containers Execution
The formicary supports executors based on Docker, Kubernetes, HTTP and Shell. You don't need to install anything for HTTP and Shell executors but Docker and Kubernetes require access to the server environment.

#### Install Docker
 - Install Docker-Community-Edition from https://docs.docker.com/engine/installation/ or 
   find installer for your OS on https://docs.docker.com/get-docker/.
 - Install Docker-Compose from https://docs.docker.com/compose/install/.

#### Install Kubernetes
 - You can use Kubernetes by installing:
   - (Preferred locally) MicroK8s from https://ubuntu.com/tutorials/install-a-local-kubernetes-with-microk8s#1-overview
   - Minikube from https://v1-18.docs.kubernetes.io/docs/tasks/tools/install-minikube/
   - Google Kubernetes Engine  (GKE) https://cloud.google.com/kubernetes-engine/
   - Azure Kubernetes Service (AKS) https://azure.microsoft.com/en-us/services/kubernetes-service/


##### Starting Microk8 on Ubuntu (preferred for local testing)
 - microk8s.status
 - microk8s.kubectl
 - microk8s.kubectl config view --raw > $HOME/.kube/config
 - copy above config to your local ~/.kube/config
 - microk8s.enable dns

##### Starting Kubernetes/Docker env
 - minikube start --driver=docker
 - minikube dashboard
 - minikube status
 - minikube ssh
 - minikube stop
 - minikube delete
 - minikube addons list

#### Starting K3 on Ubuntu
 - See https://k3s.io/ for installing k3, e.g.
```
  ssh ${HOST} 'export INSTALL_K3S_EXEC=" --no-deploy servicelb --no-deploy traefik"; \
    curl -sfL https://get.k3s.io | sh -'
  scp ${HOST}:/etc/rancher/k3s/k3s.yaml .
  sed -r 's/(\b[0-9]{1,3}\.){3}[0-9]{1,3}\b'/"${HOST_IP}"/ k3s.yaml > ~/.kube/k3s-vm-config && rm k3s.yaml
```
 - Then set environment variables for:
```
# set your host IP and name
HOST_IP=192.168.1.101
HOST=k3s
KUBECTL=kubectl --kubeconfig ~/.kube/k3s-vm-config
```
 - Optionally install https://k9scli.io/ or https://k8slens.dev/.
 -
#### Miscellaneous POD Commands
 - kubectl config view
 - kubectl cluster-info
 - kubectl get nodes
 - kubectl delete -n default pod <pod-name>
 - kubectl get pod
 - kubectl describe pod <pod-name>

### Define Test config
See [Configuration](configuration.md) for configuration of queen server and ant-workers.

### Running behind a proxy server
You can specify proxy settings by following environment variables:
`HTTP_PROXY` - http proxy
`HTTPS_PROXY` - https proxy
`NO_PROXY` - no proxy hosts

