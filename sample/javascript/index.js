const Messaging = require('./messaging')
const process = require("process")
const readline = require("readline")

const conf = {
    clientId: 'messaging-js-client',
    groupId: 'messaging-js-client',
    brokers: ['192.168.1.102:19092', '192.168.1.102:29092', '192.168.1.102:39092'],
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
    const initialMessage = {
        "user_id": "b80eef8d-3ffb-43d6-a1ed-15522d2c14c0",
        "organization_id": "",
        "job_definition_id": "b5d0e1d9-7955-425f-ac9f-de217d1cde4c",
        "job_request_id": 1,
        "job_type": "io.formicary.test.my-test-job",
        "job_type_version": "",
        "job_execution_id": "9333cfdf-5ecf-4594-9036-adb07282f1f7",
        "task_execution_id": "535f647a-8c31-4a5d-9092-89111fa2f7ac",
        "task_type": "task1",
        "task_id": "4dc875eb-5aae-4647-802c-e6d08bfdc136",
        "co_relation_id": "",
        "status": "COMPLETED",
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
        "artifacts": [],
        "warnings": [],
        "cost_factor": 0,
        "co_relation_id": "",
        "platform": "",
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
            "artifacts_dir": "/tmp/formicary-artifacts/b80eef8d-3ffb-43d6-a1ed-15522d2c14c0/job-1/task1",
            "cache": {},
            "container": {"image": "", "imageDefinition": {}},
            "helper": {"image": "", "imageDefinition": {}},
            "pod_labels": {
                "FormicaryServer": "",
                "OrganizationID": "",
                "RequestID": "1",
                "UserID": "b80eef8d-3ffb-43d6-a1ed-15522d2c14c0"
            },
            "headers": {"t1_h1": "1", "t1_h2": "true", "t1_h3": "three"}
        },
        "admin_user": false
    }
    //messaging.send(conf.inTopic, {}, initialMessage)

    rl.question('press enter to exit', () => {
        rl.close()
    })
})