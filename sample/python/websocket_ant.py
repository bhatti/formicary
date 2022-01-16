# install pip install websocket_client
import websocket
import json
import _thread as thread

HOST = "localhost:7777"
TOKEN = ""

def on_message(ws, message):
    print(message)
    req = json.loads(message)
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
            "tags": ["js", "web"],
            "methods": ["WEBSOCKET"]
        }
        ws.send(json.dumps(registration))
        print(registration)

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
