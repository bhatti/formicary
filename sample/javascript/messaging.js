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

module.exports = Messaging