import {Hono} from "https://deno.land/x/hono@v3.4.1/mod.ts";
import {Mutex} from "https://deno.land/x/async@v2.1.0/mutex.ts";
import {Buffer} from "https://deno.land/std@0.224.0/io/buffer.ts";

const app = new Hono();
const manageSocket: any = {}
const groupSocket: any = {}
const mutex = new Mutex()

const ruleManage = "manage"
const ruleAgent = "Agent"
const ruleConnector = "Connector"

const modeDirect = "direct"
const modeForward = "forward"

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
                socket.socket.onerror = (e: any) => {
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
                socket.socket.onerror = (e: any) => {
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

class MuxSocketStream {
    public socket: EmendWebsocket;
    private sessions: object;
    public readable: ReadableStream<Uint8Array>;

    constructor(socket: EmendWebsocket) {
        this.socket = socket;
        this.sessions = {};
        this.readable = new ReadableStream({
            async start(controller) {
                const buffer = new Buffer()
                let remain = -1

                const handle = async function (data: Uint8Array) {
                    // if (remain == -1) {
                    //     const len = data[0] | data[1] << 8
                    //     data = data.slice(2)
                    //     const frame = data.slice(0, len)
                    //     controller.enqueue(buffer.bytes({copy: true}));
                    // }
                    //
                    // if (remain > 0) {
                    //     if (data.byteLength > remain) {
                    //         await buffer.write(data.bytes(0, remain))
                    //     } else {
                    //         remain -= data.byteLength
                    //         await buffer.write(new Uint8Array(data))
                    //     }
                    // }
                }


                socket.socket.onmessage = async (event) => {

                };
                socket.socket.onerror = (e: any) => {
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
    }

    private handleNew(data: Uint8Array) {
    }

    private handleKeep(data: Uint8Array) {
    }

    private handleEnd() {

    }
}

async function proxy(request: any, endpoint: string) {
    const headers = new Headers({})
    headers.set('host', (new URL(endpoint)).host)
    for (const [key, value] of request.headers.entries()) {
        if (key.toLowerCase() == 'host') {
            continue
        }
        headers.set(key, value)
    }
    console.log("req url:", endpoint)
    const init: RequestInit = {
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

    const name = c.req.query("name") || ""
    const addr = c.req.query("addr") || ""
    const code = c.req.query("code") || ""
    const rule = c.req.query("rule") || ""
    const mode = c.req.query("mode") || ""

    const regex = /^([a-zA-Z0-9.]+):(\d+)$/
    if (rule === ruleConnector && mode == modeDirect && regex.test(addr)) {
        const tokens = regex.exec(addr) || []
        const hostname = tokens[1]
        const port = parseInt(tokens[2])
        console.log(`${addr} hostname: ${hostname}, port:${port}`)
        const {response, socket} = Deno.upgradeWebSocket(c.req.raw)
        socket.onerror = (e: any) => {
            console.log("socket onerror", e.message);
        }
        socket.onclose = () => {
            console.log("socket closed");
        }
        socket.onopen = () => {
            const local = new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${addr}`))
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
    if ((rule === ruleConnector || rule == ruleAgent) && name != "") {
        targetConn = manageSocket[name]
        if (!targetConn) {
            console.log("the target Agent websocket not exist", name)
            return new Response("agent not running.");
        }
    }
    console.log(`name: ${name}, code: ${code}, rule: ${rule}`)

    const {response, socket} = Deno.upgradeWebSocket(c.req.raw)
    if (rule === ruleManage) {
        socket.onopen = () => {
            manageSocket[name] = new EmendWebsocket(socket, `${rule}_${name}`)
        }
        socket.onerror = (e: any) => {
            console.log("socket onerror", e.message, socket.extensions);
            manageSocket[name] = null
        }
        socket.onclose = () => {
            console.log("socket closed", socket.extensions);
            manageSocket[name] = null
        }
        return response
    }

    await mutex.acquire();
    if (groupSocket[code]) {
        socket.onopen = () => {
            groupSocket[code].second = new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${name}`))
            const connPair = groupSocket[code]
            mutex.release()

            const first = connPair.first
            const second = connPair.second
            first.readable.pipeTo(second.writable).catch((e: ErrorEvent) => {
                console.log("socket exception", first.socket.attrs, e.message)
                groupSocket[code] = null
            })
            second.readable.pipeTo(first.writable).catch((e: ErrorEvent) => {
                console.log("socket exception", second.socket.attrs, e.message)
                groupSocket[code] = null
            })
        }
    } else {
        socket.onopen = () => {
            groupSocket[code] = {
                first: new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${name}`)),
            }
            mutex.release()
        }

        const messageData: any = {
            Code: code,
            Addr: addr,
            Network: "tcp",
        }
        if (mode == ruleAgent) {
            messageData["Mux"] = 1
        }
        const message = JSON.stringify({
            Command: 0x01,
            Data: messageData
        })
        console.log("send data:", message)
        targetConn.send(message)
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
    } else {
        endpoint = "https://www.bing.com"
    }

    return await proxy(request, endpoint)
})

Deno.serve(app.fetch);

