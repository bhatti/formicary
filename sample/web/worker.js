const Connect = function () {
    const host = document.getElementById('host').value;
    let uri = '';
    if (host.includes(':')) {
        uri = 'ws://' + host + '/ws/ants';
    } else {
        uri = 'wss://' + host + '/ws/ants';
    }

    console.log({uri, host}, 'Connecting')

    const ws = new WebSocket(uri);
    ws.onopen = function () {
        console.log({uri}, 'Connected');
        const registration = {
            'ant_id': 'sample-web',
            'tags': ['js', 'web'],
            'methods': ['WEBSOCKET']
        }
        ws.send(JSON.stringify(registration));
        console.log(new Date() + ': sent ' + JSON.stringify(registration));
    }

    ws.onmessage = function (evt) {
        console.log(evt.data);
        const msg = JSON.parse(evt.data);
        if (msg.task_retry < 2) {
            msg.ant_id = 'sample-web';
            msg.host = 'my-client';
            msg.status = 'EXECUTING';
        } else {
            msg.ant_id = 'sample-web';
            msg.host = 'my-client';
            msg.status = 'COMPLETED';
        }
        ws.send(JSON.stringify(msg));
        console.log(msg);
    }
};
