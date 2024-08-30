import {Hono} from "https://deno.land/x/hono@v3.4.1/mod.ts";
import {Mutex} from "https://deno.land/x/async@v2.1.0/mutex.ts";

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

class Buffer {
    private start: number;
    private end: number;
    private data: Uint8Array;

    constructor() {
        this.start = 0
        this.end = 0
        this.data = new Uint8Array(8192)
    }

    write(data: ArrayBufferLike, length: number) {
        for (let i = 0; i < length; i++) {
            this.data[this.end] = data[i]
            this.end++
        }
    }

    bytes(): Uint8Array {
        return this.data.slice(this.start, this.end)
    }

    reset() {
        this.start = 0
        this.end = 0
    }

    length() {
        return this.end - this.start
    }
}

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
                console.log("size", chunk.byteLength)
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
    private socket: WebSocketStream;
    private sessions: Map<number, Deno.Conn>;

    constructor(socket: WebSocketStream) {
        this.socket = socket;
        this.sessions = new Map<number, Deno.Conn>();
        this.run().catch((err) => {
            console.error("run::catch", err)
            this.socket.socket.close(1000)
        })
    }

    async run() {
        const StatusNew = 0x01
        const StatusKeep = 0x02
        const StatusEnd = 0x03
        const StatusKeepAlive = 0x04

        const OptionData = 0x01
        const OptionError = 0x02

        const Mask = 255

        let sessionID: number = 0
        let needParse = true, remainLen: number = 0, remainChunk: Uint8Array
        const buffer = new Buffer()

        for await (let chunk of this.socket.readable) {
            console.log("read from websocket", chunk.byteLength)
            if (!needParse) {
                if (remainLen > chunk.byteLength) {
                    remainLen -= chunk.byteLength
                    buffer.write(chunk, chunk.byteLength)
                    continue
                } else {
                    buffer.write(chunk, remainLen)
                    await this.sessions.get(sessionID)?.write(buffer.bytes())

                    needParse = true
                    chunk = chunk.slice(remainLen)
                    if (chunk.byteLength == 0) continue
                }
            }

            if (needParse) {
                if (remainChunk && remainChunk.byteLength > 0) {
                    const newChunk = new Uint8Array(chunk.byteLength + remainChunk.length)
                    let index = 0
                    for (let i=0; i<remainChunk.length; i++) {
                        newChunk[index++] = remainChunk[i]
                    }
                    for (let i=0; i<chunk.byteLength; i++) {
                        newChunk[index++] = chunk[i]
                    }
                    chunk = newChunk
                }

                const frameLen = chunk[1] | chunk[0] << 8
                const frame = chunk.slice(2, 2 + frameLen)

                sessionID = frame[1] | frame[0] << 8
                const status = frame[2]
                const option = frame[3]

                // StatusNew
                if (status === StatusNew) {
                    const network = frame[4];
                    const domain = (new TextDecoder()).decode(frame.slice(5));

                    const token = domain.split(':')
                    const common = {
                        id: sessionID,
                        buf: new Buffer(),
                        socket: this.socket.socket
                    }
                    console.log(`domain: ${domain}`, token[0], token[1])
                    Deno.connect({
                        hostname: token[0],
                        port: parseInt(token[1])
                    }).then(async (conn) => {
                        console.log(`network: ${network} domain: ${domain}, ${common.id}`)
                        this.sessions.set(common.id, conn)
                        await conn.readable.pipeTo(new WritableStream({
                            write(raw, controller) {
                                console.log("read from conn", raw.byteLength, common.id)
                                const N = raw.byteLength
                                let index = 0
                                while (index < N) {
                                    common.buf.reset()
                                    const size = index + 4096 < N ? 4096 : N - index
                                    const header = new Uint8Array([0, 4, 0, common.id, StatusKeep, OptionData])
                                    const length = new Uint8Array([(size >> 8) & Mask, size & Mask])
                                    common.buf.write(header, header.length)
                                    common.buf.write(length, length.length)
                                    common.buf.write(raw.slice(index, index + size), size)
                                    console.log("write to socket header, length", common.buf.length())
                                    common.socket.send(common.buf.bytes())
                                    index = index + size
                                }
                            },
                        }))
                    }).catch((err) => {
                        console.log("connect::catch", err)
                        common.buf.reset()
                        const header = new Uint8Array([0, 4, 0, common.id, StatusEnd, OptionError])
                        common.buf.write(header, header.length)
                        common.socket.send(common.buf.bytes())
                        this.sessions.delete(common.id)
                    })
                    continue
                }

                // StatusEnd
                if (status === StatusEnd) {
                    console.log(`StatusEnd end`)
                    this.sessions.delete(sessionID)
                    continue
                }

                // StatusKeepAlive
                if (status === StatusKeepAlive) {
                    console.log(`StatusKeepAlive end`)
                    continue
                }

                // StatusKeep
                if (status == StatusKeep) {
                    chunk = chunk.slice(2 + frameLen)
                    remainLen = chunk[1] | chunk[0] << 8

                    chunk = chunk.slice(2)
                    if (remainLen <= chunk.byteLength) {
                        buffer.reset()
                        buffer.write(chunk, remainLen)
                        await this.sessions.get(sessionID)?.write(buffer.bytes())

                        remainChunk = chunk.slice(remainLen)
                        remainLen = 0
                        needParse = true
                    } else {
                        buffer.reset()
                        remainLen -= chunk.byteLength
                        needParse = false
                        buffer.write(chunk, chunk.byteLength)
                    }
                    console.log(`remainLen: ${remainLen} needParse: ${needParse}`)
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
            new MuxSocketStream(new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${addr}`)))

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

