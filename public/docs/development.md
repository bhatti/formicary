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
 - queen/tasklet: defines implementation of server tasklets such as fork/await
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

- Executable
```
go build -o formicary -mod vendor ./...
```

