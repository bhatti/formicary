## Running formicary

### Running via Docker
Download docker-compose:
```
curl -LfO 'https://github.com/bhatti/formicary/blob/main/docker-compose.yaml'
```

Start docker-compose
```
docker-compose up
```

You can then verify docker containers, e.g.:
```
docker ps
```

Shutting down docker-compose
```
docker-compose down
```

### Running manually 
#### Start Minio
```
MINIO_ROOT_USER=admin MINIO_ROOT_PASSWORD=password ./minio server minio-data
```

#### Start Redis
```
redis-server
```

#### Configure queen-server and ant-worker
Configure `.formicary-ant.yaml` and `.formicary-queen.yaml` if needed. See [Configuration](configuration.md) for more configuration details.

#### Start Queen Server
```
go run main.go --config=.formicary-queen.yaml --id=formicary-server-id1 --port 7777
```

### Start Ant Worker
```
go run main.go ant --config=.formicary-ant.yaml --id=formicary-ant-id1 --port 7771 --tags "builder pulsar redis kotlin aws-lambda"
```

### Open Formicary Dashboard
Open `http://localhost:7777/dashboard` in the browser.

### Upload Job Definition

### Submit a Job

### Watch Job Execution and Results


### Running behind a proxy server
You can specify proxy settings by following environment variables:
`HTTP_PROXY` - http proxy
`HTTPS_PROXY` - https proxy
`NO_PROXY` - no proxy hosts

Alternatively, you can also specify `proxy_url` of common configuration, e.g.
```
common:
    id: my-ant-id
    user_agent: "formicary-agent"
    proxy_url: "https://myproxy"
```