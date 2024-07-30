import {Hono} from "https://deno.land/x/hono@v3.4.1/mod.ts";
import {Mutex} from "https://deno.land/x/async@v2.1.0/mutex.ts";

const app = new Hono();
const manageSocket = {}
const groupSocket = {}
const mutex = new Mutex()

class EmendWebsocket {
    public socket: WebSocket
    public attrs: string

    constructor(socket: WebSocket, attrs: string) {
        this.socket = socket
        this.attrs = attrs
    }

    close(code?: number, reason?: string) {
        this.socket.close(code, reason)
    }

    send(chunk: string | ArrayBufferLike | ArrayBuffer) {
        this.socket.send(chunk)
    }
}

class WebSocketStream {
    public socket: EmendWebsocket;
    public readable: ReadableStream<Uint8Array>;
    public writable: WritableStream<Uint8Array>;

    constructor(socket: EmendWebsocket) {
        this.socket = socket;
        this.readable = new ReadableStream({
            async start(controller) {
                socket.socket.onmessage = (event) => {
                    console.log(socket.attrs, "read len", event.data.byteLength)
                    controller.enqueue(new Uint8Array(event.data));
                };
                socket.socket.onerror = (e) => {
                    console.log("<readable onerror>", e.message)
                    controller.error(e)
                };

                socket.socket.onclose = () => {
                    console.log("<readable onclose>:", socket.attrs)
                    controller.close()
                }
            },
            pull(controller) {
            },
            cancel() {
                socket.close(1000, socket.attrs + "readable cancel");
            },
        });
        this.writable = new WritableStream({
            start(controller) {
                socket.socket.onerror = (e) => {
                    console.log("<writable onerror>:", e.message)
                }
                socket.socket.onclose = () => {
                    console.log("<writable onclose>:" + socket.attrs)
                }
            },
            write(chunk) {
                socket.send(chunk);
            },
            close() {
                socket.close(1000, socket.attrs + "writable close");
            },
            abort(e) {
                socket.close(1006, socket.attrs + "writable abort");
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
            manageSocket[uid] = new EmendWebsocket(socket, `${rule}_${uid}`)
        }
        socket.onerror = (e) => {
            console.log("socket onerror", e.message, socket.extensions);
            manageSocket[uid] = null
        }
        socket.onclose = () => {
            console.log("socket closed", socket.extensions);
            manageSocket[uid] = null
        }
        return response
    }

    await mutex.acquire();
    if (groupSocket[code]) {
        socket.onopen = () => {
            groupSocket[code].second = new EmendWebsocket(socket, `${rule}_${code}_${uid}`)
            const connPair = groupSocket[code]
            mutex.release()

            const first = new WebSocketStream(connPair.first)
            const second = new WebSocketStream(connPair.second)

            first.readable.pipeTo(second.writable)
            second.readable.pipeTo(first.writable)
        }
    } else {
        socket.onopen = () => {
            groupSocket[code] = {
                first: new EmendWebsocket(socket, `${rule}_${code}_${uid}`),
            }
            mutex.release()
        }

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

