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
const modeDirectMux = "directMux"
const modeForwardMux = "forwardMux"

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
    public socket: WebSocketStream;
    private sessions: Map<number, Deno.Conn>;

    constructor(socket: WebSocketStream) {
        this.socket = socket;
        this.sessions = new Map<number, Deno.Conn>();
        this.run().catch((err) => {
            console.error("catch", err)
        })
    }

    async run() {
        const StatusNew = 0x01
        const StatusKeep = 0x02
        const StatusEnd = 0x03
        const StatusKeepAlive = 0x04

        const OptionData = 0x01
        const OptionError = 0x02

        const handleNew = (network: number, data: Uint8Array) => {

        }

        const handleKeep = (data: Uint8Array) => {
        }

        const handleEnd = () => {
        }

        let sessionID: number = -1, option: number;

        let needParse = true
        let needDataLen: number = 0
        const buffer = new Buffer()
        for await (let chunk of this.socket.readable) {
            console.log("read from socket", chunk.byteLength)
            if (!needParse) {
                if (needDataLen > chunk.byteLength) {
                    needDataLen -= chunk.byteLength
                    buffer.writeSync(chunk)
                    continue
                } else {
                    buffer.writeSync(chunk.slice(0, needDataLen))
                    console.log("write to conn", sessionID)
                    await this.sessions.get(1)?.write(buffer.bytes())

                    buffer.reset()
                    needDataLen = 0
                    needParse = true
                    chunk = chunk.slice(needDataLen)
                    if (chunk.byteLength == 0) continue
                }
            }

            if (needParse) {
                const frameLen = chunk[1] | chunk[0] << 8
                const frame = chunk.slice(2, 2 + frameLen)

                sessionID = frame[1] | frame[0] << 8
                const status = frame[2]
                option = frame[3]

                // StatusNew
                if (status === StatusNew) {
                    const network = frame[4];
                    const decoder = new TextDecoder();
                    const domain = decoder.decode(frame.slice(5));

                    Deno.connect({
                        hostname: "127.0.0.3",
                        port: 22
                    }).then(async (conn) => {
                        const id = sessionID
                        console.log(`network: ${network} domain: ${domain}, ${id}`)
                        this.sessions.set(id, conn)
                        const buf = new Buffer()
                        const socket = this.socket.socket
                        const writeable = new WritableStream({
                            start(controller) {
                            },
                            write(raw, controller) {
                                console.log("read from conn", raw.byteLength, id)
                                const header = new Uint8Array([0, 4, 0, id, StatusKeep, OptionData])
                                const length = new Uint8Array([(raw.byteLength >> 8) & 127, (raw.byteLength) & 127])
                                buf.writeSync(header)
                                buf.writeSync(length)
                                buf.writeSync(raw)
                                console.log("write to socket header, length", header, length, id, buf.length)
                                socket.send(buf.bytes())
                                buf.reset()
                            },
                            close() {
                                socket.close(1000, socket.attrs + "writable close");
                            },
                            abort(e) {
                                socket.close(1006, socket.attrs + "writable abort");
                            },
                        })

                        conn.readable.pipeTo(writeable).catch(() => {

                        })
                    }).catch((err) => {
                        console.log("err", err)
                    })
                    continue
                }

                // StatusEnd
                if (status === StatusEnd) {
                    console.log(`StatusEnd end`)
                    continue
                }

                // StatusKeepAlive
                if (status === StatusKeepAlive) {
                    console.log(`StatusKeepAlive end`)
                    continue
                }

                // StatusKeep
                if (status == StatusKeep) {
                    let data = chunk.slice(2 + frameLen)
                    needDataLen = data[1] | data[0] << 8
                    data = data.slice(2)
                    console.log(`needDataLen: ${needDataLen} ${data.byteLength}`)
                    needDataLen -= data.byteLength
                    buffer.writeSync(data)
                    if (needDataLen === 0) {
                        await this.sessions.get(1)?.write(buffer.bytes())
                        buffer.reset()
                        continue
                    }

                    needParse = !(needDataLen > 0)
                    console.log(`needDataLen: ${needDataLen} needParse: ${needParse}`)
                }
            }
        }
    }

}

function proxy(request: any, endpoint: string) {
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

    return fetch(endpoint, init).then((response) => {
        return new Response(response.body, response)
    })
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
    if ([ruleConnector, ruleAgent].includes(rule) && [modeDirect, modeDirectMux].includes(mode) && regex.test(addr)) {
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
            new MuxSocketStream(local)

            // Deno.connect({
            //     port: port,
            //     hostname: hostname,
            // }).then((remote) => {
            //
            //
            //     local.readable.pipeTo(remote.writable).catch((e) => {
            //         console.log("socket exception", e.message)
            //     })
            //     remote.readable.pipeTo(local.writable).catch((e) => {
            //         console.log("socket exception", e.message)
            //     })
            // }).catch((e) => {
            //     local.socket.close()
            //     console.log("socket exception", e.message)
            // })
        }
        return response
    }

    let targetConn
    if ([ruleConnector, ruleAgent].includes(rule) && name != "") {
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
            delete manageSocket[name]
        }
        socket.onclose = () => {
            console.log("socket closed", socket.extensions);
            delete manageSocket[name]
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

        const messageData = {
            Code: code,
            Addr: addr,
            Network: "tcp",
            Mux: [modeDirectMux, modeDirectMux].includes(mode)
        }
        const message = JSON.stringify({
            Command: 0x01,
            Data: messageData
        })
        console.log("send data:", message)
        console.log("xxx", targetConn)
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

