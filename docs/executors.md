## Executors

### Shell or Local Executor
The shell executor forks a shell process from ant work for executing commands defined under `script`. It does not
require any additional configuration, but it's recommended to use a unique user for the ant worker with proper
permissions because a user may invoke any command on the machine.

### REST
RES API Executor invokes external HTTP APIs using GET, POST, PUT or DELTE actions, e.g.
```
job_type: http-job
tasks:
- task_type: get
  method: HTTP_GET
  url: https://jsonplaceholder.typicode.com/todos/1
  on_completed: post
- task_type: post
  method: HTTP_POST_JSON
  url: https://jsonplaceholder.typicode.com/todos
  on_completed: put
- task_type: put
  method: HTTP_PUT_JSON
  url: https://jsonplaceholder.typicode.com/todos/1
  on_completed: delete
- task_type: delete
  method: HTTP_DELETE
  url: https://jsonplaceholder.typicode.com/todos/1
```

### Websockets
Websockets method allows connecting browser or python/go/java/etc ant workers to execute the tasks, e.g.
```
job_type: web-job
tasks:
- task_type: process
  method: WEBSOCKET
  tags:
    - web
    - js
```

The web or client uses websocket clients register with the server, e.g.
```
    const ws = new WebSocket(uri);
    ws.onopen = function () {
        const registration = {
            'ant_id': 'sample-web',
            'tags': ['js', 'web'],
            'methods': ['WEBSOCKET']
        }
        ws.send(JSON.stringify(registration));
    }

    ws.onmessage = function (evt) {
        const msg = JSON.parse(evt.data);
        // handle message
        msg.status = 'COMPLETED';
        ws.send(JSON.stringify(msg));
    }
```

Following is an example of python ant worker:
```
import websocket
import json
import _thread as thread

HOST = "localhost:7777"
TOKEN = ""

def on_message(ws, message):
    req = json.loads(message)
    # process message
    req["status"] = "COMPLETED"
    ws.send(json.dumps(req))

def on_error(ws, error):
    print(error)

def on_close(ws):
    print("### closed ###")

def on_open(ws):
    def run(*args):
        registration = {
            "ant_id": "sample-python",
            "tags": ["python", "web"],
            "methods": ["WEBSOCKET"]
        }
        ws.send(json.dumps(registration))

    thread.start_new_thread(run, ())

if __name__ == "__main__":
    headers = {
            "Authorization": TOKEN
            }
    ws = websocket.WebSocketApp("wss://" + HOST + "/ws/ants",
                              header=headers,
                              on_open = on_open,
                              on_message = on_message,
                              on_error = on_error,
                              on_close = on_close)
    ws.run_forever()
```


### Docker
The Docker executor starts a main container for executing script named after job/task name and a helper container
wth `-helper` suffix for managing artifacts. The initial docker config are defined by the ant config that are available for all jobs such as:

- helper_image - helper image
- host - Docker host
- registry server - docker registry
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
docker:
  registry:
    registry: docker-registry-server
    username: docker-registry-user
    password: docker-registry-pass
    pull_policy: if-not-present
  host: kubernetes-host
```

Above configuration applies to all jobs, but a docker task can define following properties for each job-definition:

- name - the name of task that is used for pod-name
- environment - environment variables to set within the container
- working_directory - for script execution
- container - main container to execute, which defines:
    - image
    - image_definition
- network_mode
- host_network e.g.,

```yaml
name: task1
method: DOCKER
environment:
  AWS-KEY: Mykey
container:
  image: ubuntu:16.04
privileged: true
network_mode: mod1
host_network: true
```


### Kubernetes

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
    projected:
      - name: oidc-token
        mount_path: /var/run/sigstore/cosign
        sources:
          - service_account_token:
            path: oidc-token
            expiration_seconds: 600
            audience: sigstore
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
      projected:
        - name: oidc-token
          mount_path: /var/run/sigstore/cosign
          sources:
            - service_account_token:
              path: oidc-token
              expiration_seconds: 600
              audience: sigstore
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

### Customized
You can implement a customized executor by subscribing to the messaging queue, e.g. here is a sample messaging executor:
```go
// MessagingHandler structure
type MessagingHandler struct {
	id            string
	requestTopic  string
	responseTopic string
	queueClient   queue.Client
}

// NewMessagingHandler constructor
func NewMessagingHandler(
	id string,
	requestTopic string,
	responseTopic string,
	queueClient queue.Client,
) *MessagingHandler {
	return &MessagingHandler{
		id:            id,
		requestTopic:  requestTopic,
		responseTopic: responseTopic,
		queueClient:   queueClient,
	}
}

func (h *MessagingHandler) Start(
	ctx context.Context,
) (err error) {
	return h.queueClient.Subscribe(
		ctx,
		h.requestTopic,
		h.id,
		make(map[string]string),
		true, // shared subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			err = h.execute(ctx, event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "MessagingHandler",
					"Payload":   string(event.Payload),
					"Target":    h.id,
					"Error":     err}).Error("failed to execute")
				return err
			}
			return nil
		},
	)
}

// Stop stops subscription
func (h *MessagingHandler) Stop(
	ctx context.Context,
) (err error) {
	return h.queueClient.UnSubscribe(
		ctx,
		h.requestTopic,
		h.id,
	)
}

// execute incoming request
func (h *MessagingHandler) execute(
	ctx context.Context,
	reqPayload []byte) (err error) {
	var req *types.TaskRequest
    req, err = types.UnmarshalTaskRequest(h.antCfg.EncryptionKey, reqPayload)
	if err != nil {
		return err
	}
	resp := types.NewTaskResponse(req)

	// Implement business logic below
	epoch := time.Now().Unix()
	if epoch%2 == 0 {
		resp.Status = types.COMPLETED
	} else {
		resp.ErrorCode = "ERR_MESSAGING_WORKER"
		resp.ErrorMessage = "mock error for messaging client"
		resp.Status = types.FAILED
	}
	resp.AddContext("epoch", epoch)

	// Send back reply
    resPayload, err := resp.Marshal(h.antCfg.EncryptionKey)
	if err != nil {
		return err
	}
	_, err = h.queueClient.Send(
		ctx,
		h.responseTopic,
		make(map[string]string),
		resPayload,
		false)
	return err
}
```

Here is an equivalent implementation of messaging ant worker in Javascript:
```javascript
const {Kafka} = require('kafkajs')
const assert = require('assert')

// Messaging helper class for communication with the queen server
class Messaging {
    constructor(config) {
        assert(config.clientId, `clientId is not specified`)
        assert(config.brokers.length, 'brokers is not specified')
        config.reconnectTimeout = config.reconnectTimeout || 10000
        this.config = config
        this.kafka = new Kafka(config)
        this.producers = {}
        console.info({config}, `initializing messaging helper`)
    }

    // send a message to the topic
    async send(topic, headers, message) {
        const producer = await this.getProducer(topic)
        await producer.send({
            topic: topic,
            messages: [
                {
                    key: headers['key'],
                    headers: headers,
                    value: typeof message == 'object' ? JSON.stringify(message) : message,
                }
            ]
        })
        console.debug({topic, message}, `sent message to ${topic}`)
    }

    // subscribe to topic with given callback
    async subscribe(topic, cb, tries = 0) {
        //const meta = await this.getTopicMetadata(topic)
        const consumer = this.kafka.consumer({groupId: this.config.groupId})
        try {
            await consumer.connect()
            const subConfig = {topic: topic, fromBeginning: false}
            await consumer.subscribe(subConfig)
            const handle = (message, topic, partition) => {
                const headers = message.headers || {}
                if (topic) {
                    headers.topic = topic
                }
                if (partition) {
                    headers.partition = partition.toString()
                }

                headers.key = (message.key || '').toString()
                headers.timestamp = message.timestamp
                headers.size = (message.size || '0').toString()
                headers.attributes = (message.attributes || '[]').toString()
                headers.offset = message.offset
                cb(headers, JSON.parse(message.value || ''))
            }

            console.info({topic, groupId: this.config.groupId},
                `subscribing consumer to ${topic}`)
            await consumer.run({
                eachBatchAutoResolve: false,
                eachBatch: async ({batch, resolveOffset, heartbeat, isRunning, isStale}) => {
                    for (let message of batch.messages) {
                        if (!isRunning()) break
                        if (isStale()) continue
                        handle(message, topic)
                        resolveOffset(message.offset) // commit
                        await heartbeat()
                    }
                }
            })

            return () => {
                console.info({topic, groupId: this.config.groupId}, `closing consumer`)
                consumer.disconnect()
            }
        } catch (e) {
            console.warn(
                {topic, error: e, config: this.config},
                `could not subscribe, will try again`)
            tries++
            return new Promise((resolve) => {
                setTimeout(() => {
                    resolve(this.subscribe(topic, cb, tries))
                }, Math.min(tries * 1000, this.config.reconnectTimeout + 5000))
            })
        }
    }

    // getTopicMetadata returns partition metadata for the topic
    async getTopicMetadata(topic, groupId, tries = 0) {
        const admin = this.kafka.admin()
        try {
            await admin.connect()
            const response = {}
            if (groupId) {
                response.offset = await admin.fetchOffsets({groupId, topic})
            } else {
                response.offset = await admin.fetchTopicOffsets(topic)
            }
            const meta = await admin.fetchTopicMetadata({topics: [topic]})
            if (meta && meta.topics && meta.topics[0].partitions) {
                response.partitions = meta.topics[0].partitions
            }
            response.topics = await admin.listTopics()
            response.groups = (await admin.listGroups())['groups']
            return response
        } catch (e) {
            console.warn(
                {topic, error: e, config: this.config},
                `could not get metadata, will try again`)
            tries++
            return new Promise((resolve) => {
                setTimeout(() => {
                    resolve(this.getTopicMetadata(topic, groupId, tries))
                }, Math.min(tries * 1000, this.config.reconnectTimeout))
            })
        }
    }

    // getProducer returns producer for the topic
    async getProducer(topic, tries = 0) {
        const producer = this.kafka.producer()
        try {
            await producer.connect()
            this.producers[topic] = producer
            console.info({topic, tries}, `adding producer`)
            return producer
        } catch (e) {
            console.warn(
                {topic, error: e, config: this.config},
                `could not get messaging producer, will try again`)
            tries++
            return new Promise((resolve) => {
                setTimeout(() => {
                    resolve(this.getProducer(topic, tries))
                }, Math.min(tries * 1000, this.config.reconnectTimeout))
            })
        }
    }

    // closeProducers closes producers
    async closeProducers() {
        Object.entries(this.producers).forEach(([topic, producer]) => {
            console.info({topic}, `closing producer`)
            try {
                producer.disconnect()
            } catch (e) {
            }
        })
        this.producers = {}
    }

}


const Messaging = require('./messaging')
const process = require("process")
const readline = require("readline")

const conf = {
    clientId: 'messaging-js-client',
    groupId: 'messaging-js-client',
    brokers: ['127.0.0.1:19092', '127.0.0.1:29092', '127.0.0.1:39092'],
    inTopic: 'formicary-message-ant-request',
    outTopic: 'formicary-message-ant-response',
    connectionTimeout: 10000,
    requestTimeout: 10000,
}

const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
})

rl.on("close", () => {
    process.exit(0)
});

const messaging = new Messaging(conf)
messaging.subscribe(conf.inTopic, async (headers, msg) => {
    console.log({headers, msg}, `received from ${conf.inTopic}`)
    msg.status = 'COMPLETED'
    msg.taskContext = {'key1': 'value1'}
    messaging.send(conf.outTopic, headers, msg)
}).then(() => {
    rl.question('press enter to exit', () => {
        rl.close()
    })
})
```

Here is a sample job definition that uses `MESSAGING` executor:
```yaml
job_type: messaging-job
timeout: 60s
tasks:
- task_type: trigger
  method: MESSAGING
  messaging_request_queue: formicary-message-ant-request
  messaging_reply_queue: formicary-message-ant-response
```

### Task Request
The task request is sent to the ant work for executing the work and includes following properties:
```
    {
        "user_id": "uuid",
        "organization_id": "uuid",
        "job_definition_id": "uuid",
        "job_request_id": 1,
        "job_type": "my-test-job",
        "task_type": "task1",
        "job_execution_id": "uuid",
        "task_execution_id": "uuid",
        "co_relation_id": "uuid",
        "tags": [],
        "platform": "linux",
        "action": "EXECUTE",
        "job_retry": 0,
        "task_retry": 0,
        "allow_failure": false,
        "before_script": ["t1_cmd1", "t1_cmd2", "t1_cmd3"],
        "after_script": ["t1_cmd1", "t1_cmd2", "t1_cmd3"],
        "script": ["t1_cmd1", "t1_cmd2", "t1_cmd3"],
        "timeout": 0,
        "variables": {
            "jk1": {"name": "", "value": "jv1", "secret": false},
            "jk2": {"name": "", "value": {"a": 1, "b": 2}, "secret": false},
        },
        "executor_opts": {
            "name": "frm-1-task1-0-0-6966",
            "method": "KUBERNETES",
            "container": {"image": "", "imageDefinition": {}},
            "helper": {"image": "", "imageDefinition": {}},
            "headers": {"t1_h1": "1", "t1_h2": "true", "t1_h3": "three"}
        },
    }

```

### Task Response
The task response is sent back by the ant work after executing the work and includes following properties:
```
    {
        "job_request_id": 1,
        "job_type": "my-test-job",
        "task_type": "task1",
        "task_execution_id": "uuid",
        "co_relation_id": "uuid",
        "status": "COMPLETED",
        "co_relation_id": "uuid",
        "ant_id": "",
        "host": "",
        "namespace": "",
        "tags": [],
        "error_message": "",
        "error_code": "",
        "exit_code": "",
        "exit_message": "",
        "failed_command": "",
        "task_context": {},
        "job_context": {},
        "warnings": [],
        "cost_factor": 0,
    }

```
