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
            write(chunk, controller) {
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

async function proxy(request, endpoint) {
    const headers = new Headers({})
    headers.set('host', (new URL(endpoint)).host)
    for (const [key, value] of request.headers.entries()) {
        if (key.toLowerCase() == 'host') {
            continue
        }
        headers.set(key, value)
    }
    console.log("req url:", endpoint)
    let init = {
        method: request.method,
        headers: headers
    }
    if (['POST', 'PUT'].includes(request.method.toUpperCase())) {
        init.body = request.body
    }
    const response = await fetch(endpoint, init)
    return new Response(response.body, response)
}

app.get("/api/ssh", async (c) => {
    const upgrade = c.req.headers.get("upgrade") || "";
    if (upgrade.toLowerCase() != "websocket") {
        return new Response("request isn't trying to upgrade to websocket.");
    }

    const uid = c.req.query("uid")
    const code = c.req.query("code")
    const rule = c.req.query("rule")

    const regex = /^([a-zA-Z0-9.]+):(\d+)$/
    if (rule === "Connector" && regex.test(uid)) {
        const tokens = regex.exec(uid)
        const hostname = tokens[1]
        const port = parseInt(tokens[2])
        console.log(`${uid} hostname: ${hostname}, port:${port}`)
        const {response, socket} = Deno.upgradeWebSocket(c.req.raw)
        socket.onerror = (e) => {
            console.log("socket onerror", e.message);
        }
        socket.onclose = () => {
            console.log("socket closed");
        }
        socket.onopen = () => {
            const local = new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${uid}`))
            Deno.connect({
                port: port,
                hostname: hostname,
            }).then((remote) => {
                local.readable.pipeTo(remote.writable).catch((e) => {
                    console.log("socket exception", e.message)
                })
                remote.readable.pipeTo(local.writable).catch((e) => {
                    console.log("socket exception", e.message)
                })
            }).catch((e) => {
                local.socket.close()
                console.log("socket exception", e.message)
            })
        }
        return response
    }

    let targetConn
    if (rule === "Connector") {
        targetConn = manageSocket[uid]
        if (!targetConn) {
            console.log("the target Agent websocket not exist", uid)
            return new Response("agent not running.");
        }
    }
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
            groupSocket[code].second = new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${uid}`))
            const connPair = groupSocket[code]
            mutex.release()

            const first = connPair.first
            const second = connPair.second
            first.readable.pipeTo(second.writable).catch((e) => {
               console.log("socket exception", first.socket.attrs, e.message)
               groupSocket[code] = null
            })
            second.readable.pipeTo(first.writable).catch((e) => {
               console.log("socket exception", second.socket.attrs, e.message)
               groupSocket[code] = null
            })
        }
    } else {
        socket.onopen = () => {
            groupSocket[code] = {
                first: new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${uid}`)),
            }
            mutex.release()
        }
        const data = JSON.stringify({
            Command: 0x01,
            Data: {Code: code}
        })
        console.log("send data:", data)
        targetConn.send(data)
    }

    return response;
})

app.on(['GET', 'DELETE', 'HEAD', 'OPTIONS', 'PUT', 'POST'], "*", async (c) => {
    const request = c.req.raw
    const url = new URL(request.url)
    const path = url.pathname + url.search

    let endpoint = ""
    if (path.startsWith("/https://") || path.startsWith("/http://")) {
        endpoint = path.substring(1)
    }  else {
        endpoint = "https://www.bing.com"
    }

    return await proxy(request, endpoint)
})

Deno.serve(app.fetch);

