## Development

### Code Structure

#### Common
 - common/artifacts: abstracts artifact upload and download
 - common/async: common async library
 - common/cache: defines caching interfaces on top of redis 
 - common/crypto: defines crypto helper functions
 - common/events: defines lifecycle and other events for jobs, tasks, containers, etc. 
 - common/health: monitors health of dependent services
 - common/queue: abstracts messaging communication between sender/receiver or pub/sub
 - common/tasklet: abstracts base classes of tasklet for serving requests
 - common/types: common domain classes
 - common/utils: common utility
 - common/web: abstracts HTTP client and server

#### Queen Server
 - queen/config: defines configuration of server
 - queen/controller: defines API controllers
 - queen/controller/admin: defines Dashboard controllers for administration
 - queen/fsm: defines state machines for jobs and tasks
 - queen/gateway: defines event gateway
 - queen/launcher: defines job launcher
 - queen/manager: defines managers for artifacts and jobs
 - queen/repository: defines data access repositories
 - queen/resource: defines resource manager
 - queen/scheduler: defines job launcher
 - queen/stats: manages job metrics
 - queen/supervisor: defines job and task supervisors
 - queen/tasklet: defines implementation of server tasklets such as fork/await and artifacts expiration
 - queen/types: defines server domain classes
 - queen/utils: defines server utility functions

#### Ant Worker
- ants/config: defines configuration of ant worker
- ants/controller: defines API controllers for ant worker
- ants/executor: defines common executor interfaces and structures
- ants/executor/docker: implements executor for docker
- ants/executor/http: implements executor for HTTP
- ants/executor/kubernetes: implements executor for Kubernetes
- ants/executor/shell: implements executor for docker Shell
- ants/handler: defines request handler for incoming requests
- ants/registry: defines registry of containers
- ants/transfer: manages object download and uploads for artifacts and caching
- sample: sample code such as messaging ant worker

#### HTML Views/Assets
 - public/assets: assets for css, images, javascript
 - public/views: HTML views

#### Docs
 - docs: general documentation
 - docs/examples: examples of job definitions

### Run Tests
```
CGO_ENABLED=1 go test -p 1  -mod vendor ./... -json > go-test-report.json
CGO_ENABLED=1 go test -p 1  -mod vendor ./... -coverprofile=go-test-coverage.out
```

- Run Test Coverage
```
go tool cover -html=go-test-coverage.out
```

- Lint Errors
```
go get -u golang.org/x/lint/golint
~/go/bin/golint -set_exit_status ./..
```

- Vet Errors
```
go vet ./... 2> go-vet-report.out
```

- Adding vendor dependencies
```
go mod vendor
```

- DB Migrations

```
goose mysql "formicary_user_dev:formicary_pass@/formicary_dev?parseTime=true" up
```

- Delete Kafka topics
```
SERVER=127.0.0.1:32181
kafka-topics --zookeeper $SERVER --topic formicary-queue-fork-job-tasklet-topic --delete
...
```

- Create Kafka topics
```
SERVER=127.0.0.1:32181

kafka-topics --zookeeper $SERVER --topic formicary-topic-job-webhook-lifecycle --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-task-webhook-lifecycle --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-container-lifecycle --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-job-definition-lifecycle --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-job-request-lifecycle --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-logs --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-health-error --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-ant-registration --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-task-ant-registration --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-task-reply --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-job-execution-lifecycle --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-task-execution-lifecycle --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-fork-job-tasklet --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-wait-fork-job-tasklet --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-ant-request --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-ant-reply --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-topic-job-scheduler-leader --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-job-execution-launch-queen1 --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-queue-job-execution-launch-anon-local --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-server-id1-artifact-expiration-tasklet --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-message-ant-request --create --partitions 1 --replication-factor 1
kafka-topics --zookeeper $SERVER --topic formicary-message-ant-response --create --partitions 1 --replication-factor 1

```

- Executable
```
go mod tidy
go build -o formicary -mod vendor ./...
```

### Dump stack trace of all goroutines
Find the golang process, e.g. `ps -ef|grep main`, then send `SIGHUP` or 1 such as `kill -1 <pid>` to dump the stack trace of goroutines.

### Graceful shutdown 
Find the golang process, e.g. `ps -ef|grep main`, then send `SIGQUIT` or 3 such as `kill -3 <pid>` to shutdown process gracefully so that it allows executing jobs/tasks
to complete while not receiving new jobs/tasks. In order to reduce any shutdown, you can start another process that can receive new requests at the same time.
