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
    await messaging.send(conf.outTopic, headers, msg)
}).then(() => {
    rl.question('press enter to exit', () => {
        rl.close()
    })
})