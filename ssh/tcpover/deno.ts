import {Hono} from "https://deno.land/x/hono@v3.4.1/mod.ts";
import {Mutex} from "https://deno.land/x/async@v2.1.0/mutex.ts";

const app = new Hono();
const manageSocket = {}
const groupSocket = {}
const mutex = new Mutex()

class WebSocketStream {
    public socket: WebSocket;
    public readable: ReadableStream<Uint8Array>;
    public writable: WritableStream<Uint8Array>;

    constructor(socket: WebSocket) {
        this.socket = socket;
        this.readable = new ReadableStream({
            start(controller) {
                socket.onmessage = function ({data}) {
                    console.log("read len", data)
                    controller.enqueue(data);
                };
                socket.onerror = (e) => {
                    socket.close()
                    console.log("readable onerror", e)
                    // controller.error(e)
                };
                socket.onclose = () => controller.close();
            },
            cancel() {
                console.log("readable cancel")
                socket.close();
            },
        });
        this.writable = new WritableStream({
            start(controller) {
                socket.onerror = (e) => {
                    console.log("writable onerror", e)
                    // controller.error(e);
                }
            },
            write(chunk) {
                socket.send(chunk);
            },
            close() {
                console.log("writable close")
                socket.close();
            },
            abort(e) {
                console.log("writable abort", e)
                socket.close();
            },
        });
    }
}

async function proxy(c, prefix, endpoint) {
    const request = c.req.raw
    const url = new URL(request.url)
    const path = url.pathname + url.search

    if (path.startsWith("/https://") || path.startsWith("/http://")) {
        endpoint = path.substring(1)
    } else if (path.startsWith(prefix)) {
        endpoint = endpoint + path
    }

    request.headers['host'] = (new URL(endpoint)).host
    console.log("req url:", endpoint)
    let init = {
        method: request.method,
        headers: request.headers
    }
    if (['POST', 'PUT'].includes(request.method.toUpperCase())) {
        init.body = request.body
    }

    const response = await fetch(endpoint, init)
// clone the response to return a response with modifiable headers
    return new Response(response.body, response)
}

app.get("/", async (c) => {
    const upgrade = c.req.headers.get("upgrade") || "";
    if (upgrade.toLowerCase() != "websocket") {
        return new Response("request isn't trying to upgrade to websocket.");
    }

    const uid = c.req.query("uid")
    const code = c.req.query("code")
    const rule = c.req.query("rule")
    console.log(`uid: ${uid}, code: ${code}, rule: ${rule}`)


    const {response, socket} = Deno.upgradeWebSocket(c.req.raw)
    if (rule === "manage") {
        socket.onopen = () => {
            console.log("socket opened");
            manageSocket[uid] = socket
        }
        socket.onerror = (e) => {
            console.log("socket onerror", e.message, uid);
            manageSocket[uid] = null
        }
        socket.onclose = () => {
            console.log("socket closed", uid);
            manageSocket[uid] = null
        }
        return response
    }

    await mutex.acquire();
    if (groupSocket[code]) {
        groupSocket[code].second = socket
        const connPair = groupSocket[code]
        mutex.release()
        connPair.second.onopen = () => {
            const first = new WebSocketStream(connPair.first)
            const second = new WebSocketStream(connPair.second)

            first.readable.pipeTo(second.writable)
            second.readable.pipeTo(first.writable)
        }
    } else {
        groupSocket[code] = {
            first: socket,
        }
        mutex.release()

        const targetConn = manageSocket[uid]
        const data = JSON.stringify({
            Command: 0x01,
            Data: {Code: code}
        })
        console.log("send data:", data)
        targetConn.send(data)
    }

    return response;
})

Deno.serve(app.fetch);

