## Installation

### Setup Database
The formicary by default uses sqlite3 but supports other relational database such as postgres, mysql and mssql. 
You don't need to setup anything if you are using sqlite but other databases will require you to create the database,
users and permissions, e.g.

#### Create Database
 - Use database specific commands to create database, users and permission.

#### DB Migrations
The migrations are automatically run when using sqlite3, however other databases will require running migrations explicitly.
The formicary uses `goose` for db migration that can be installed via:
```
 go get -u github.com/pressly/goose/cmd/goose
```
Then you can run migrations such as:
```
goose mysql "formicary_user_dev:formicary_pass@/formicary_dev?parseTime=true" up
```

### Start Minio
The formicary uses Minio for objecto-store that you can install from `https://docs.min.io/minio/baremetal/tutorials/minio-installation.html`.
Then start Minio server as:
 - mkdir -p minio-data
 - MINIO_ROOT_USER=admin MINIO_ROOT_PASSWORD=password ./minio server minio-data

### Messaging
The formicary uses messaging queues to communication between queen server and ant workers, you can use Redis or Apache Pulsar for messaging, e.g.
#### Redis
Start Redis
 - redis-server

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
 - You can use Minikube for Kubernetes by installing it from https://v1-18.docs.kubernetes.io/docs/tasks/tools/install-minikube/,
 MicroK8s from https://ubuntu.com/tutorials/install-a-local-kubernetes-with-microk8s#1-overview or other implementation.

##### Starting Kubernetes/Docker env
 - minikube start --driver=docker
 - minikube dashboard
 - minikube status
 - minikube ssh
 - minikube stop
 - minikube delete
 - minikube addons list

##### Starting Microk8 on Ubuntu
 - microk8s.status
 - microk8s.kubectl
 - microk8s.kubectl config view --raw > $HOME/.kube/config
 - copy above config to your local ~/.kube/config

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

