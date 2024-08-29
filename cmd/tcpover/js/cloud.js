// @ts-ignore
import {connect} from 'cloudflare:sockets';

class Lock {
    constructor() {
        this.queue = [];
        this.acquired = false;
    }

    async acquire() {
        if (!this.acquired) {
            this.acquired = true;
        } else {
            return new Promise((resolve, _) => {
                this.queue.push(resolve);
            });
        }
    }

    async release() {
        if (this.queue.length === 0 && this.acquired) {
            this.acquired = false;
            return;
        }
        const continuation = this.queue.shift();
        return new Promise((res) => {
            continuation();
            res();
        });
    }
}

class EmendWebsocket {
    constructor(socket, attrs) {
        this.socket = socket
        this.attrs = attrs
    }

    close(code, reason) {
        this.socket.close(code, reason)
    }

    send(chunk) {
        this.socket.send(chunk)
    }
}

class Buffer {
    constructor() {
        this.start = 0
        this.end = 0
        this.data = new Uint8Array(8192)
    }

    write(data, length) {
        for (let i = 0; i < length; i++) {
            this.data[this.end] = data[i]
            this.end++
        }
    }

    bytes() {
        return this.data.slice(this.start, this.end)
    }

    reset() {
        this.start = 0
        this.end = 0
    }
}

class WebSocketStream {
    constructor(socket) {
        this.socket = socket;
        this.readable = new ReadableStream({
            async start(controller) {
                socket.socket.addEventListener("message", (event) => {
                    controller.enqueue(new Uint8Array(event.data));
                });
                socket.socket.addEventListener("error", (e) => {
                    console.log("<readable onerror>", e.message)
                    controller.error(e)
                });
                socket.socket.addEventListener("close", () => {
                    console.log("<readable onclose>:", socket.attrs)
                    controller.close()
                });
            },
            pull(controller) {
            },
            cancel() {
                socket.close(1000, socket.attrs + "readable cancel");
            },
        });
        this.writable = new WritableStream({
            start(controller) {
                socket.socket.addEventListener("error", (e) => {
                    console.log("<writable onerror>:", e.message)
                })
                socket.socket.addEventListener("close", () => {
                    console.log("<writable onclose>:" + socket.attrs)
                })
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
    constructor(stream) {
        this.stream = stream;
        this.sessions = {}
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

        let sessionID = 0;

        let needParse = true
        let needDataLen = 0
        const buffer = new Buffer()
        const reader = this.stream.readable.getReader()
        while (true) {
            let chunk = reader.read()
            console.log("read from websocket", chunk.byteLength)
            reader.releaseLock()

            const frameLen = chunk[1] | chunk[0] << 8
            const frame = chunk.slice(2, 2 + frameLen)

            sessionID = frame[1] | frame[0] << 8
            const status = frame[2]
            const option = frame[3]

            // StatusNew
            if (status === StatusNew) {
                const network = frame[4];
                const domain = (new TextDecoder()).decode(frame.slice(5));
                const common = {
                    id: sessionID,
                    buf: new Buffer(),
                    socket: this.stream.socket
                }

                console.log(`network: ${network} domain: ${domain}, ${common.id}`)
                const conn = connect(domain, {secureTransport: "off"})
                this.sessions[common.id] = conn
                conn.readable.pipeTo(new WritableStream({
                    start(controller) {
                    },
                    write(raw, controller) {
                        console.log("read from conn", raw.byteLength, common.id)
                        const N = raw.byteLength
                        let index = 0
                        while (index < N) {
                            common.buf.reset()

                            const size = index + 2048 < N ? 2048 : N - index
                            const header = new Uint8Array([0, 4, 0, common.id, StatusKeep, OptionData])
                            const length = new Uint8Array([(size >> 8) & Mask, size & Mask])
                            common.buf.write(header, header.length)
                            common.buf.write(length, length.length)
                            common.buf.write(raw.slice(index, index + size), size)
                            console.log("write to socket header, length", header, length, common.id, size, index, common.buf.bytes())
                            common.socket.send(common.buf.bytes())
                            index = index + size
                        }
                    },
                    close() {
                    },
                    abort(e) {
                    },
                })).catch((err) => {
                    console.log("connect::catch", err)
                    common.buf.reset()
                    const header = new Uint8Array([0, 4, 0, common.id, StatusEnd, OptionError])
                    common.buf.write(header, header.length)
                    common.socket.send(common.buf.bytes())
                    delete this.sessions[common.id]
                })

                continue
            }

            // StatusEnd
            if (status === StatusEnd) {
                console.log(`StatusEnd end`)
                delete this.sessions[sessionID]
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
                buffer.reset()
                needDataLen -= data.byteLength
                buffer.write(data, data.byteLength)
                if (needDataLen === 0) {
                    await this.sessions[sessionID]?.writable.getWriter().write(buffer.bytes())
                    this.sessions[sessionID]?.writable.getWriter().releaseLock()
                    continue
                }

                console.log(`needDataLen: ${needDataLen} needParse: ${needParse}`)

                while (needDataLen > 0) {
                    chunk = reader.read()
                    console.log("read from websocket", chunk.byteLength)
                    reader.releaseLock()
                    if (needDataLen > chunk.byteLength) {
                          needDataLen -= chunk.byteLength
                          buffer.write(chunk, chunk.byteLength)
                          continue
                    } else {
                        buffer.write(chunk, needDataLen)
                        await this.sessions[sessionID]?.writable.getWriter().write(buffer.bytes())
                        this.sessions[sessionID]?.writable.getWriter().releaseLock()

                        needDataLen = 0
                        chunk = chunk.slice(needDataLen)
                        if (chunk.byteLength == 0) {
                            chunk = reader.read()
                            console.log("read from websocket", chunk.byteLength)
                            reader.releaseLock()
                        }
                    }
                }
            }
        }
    }
}

let uuid = null;
const mutex = new Lock()

const ruleManage = "manage"
const ruleAgent = "Agent"
const ruleConnector = "Connector"

const modeDirect = "direct"
const modeForward = "forward"
const modeDirectMux = "directMux"
const modeForwardMux = "forwardMux"


function check(request) {
    if (!uuid) {
        uuid = new Date();
    }
    return new Response("code is:" + uuid.valueOf())
}

function safeCloseWebSocket(socket) {
    try {
        if (socket.readyState === 1 || socket.readyState === 2) {
            socket.close();
        }
    } catch (error) {
        console.error('safeCloseWebSocket error', error);
    }
}

async function ws(request) {
    const url = new URL(request.url);

    const name = url.searchParams.get("name")
    const addr = url.searchParams.get("addr")
    const code = url.searchParams.get("code")
    const rule = url.searchParams.get("rule")
    const mode = url.searchParams.get("mode")

    const regex = /^([a-zA-Z0-9.]+):(\d+)$/

    if ([ruleConnector, ruleAgent].includes(rule) && [modeDirect, modeDirectMux].includes(mode) && regex.test(addr)) {
        const webSocketPair = new WebSocketPair();
        const [client, webSocket] = Object.values(webSocketPair);
        webSocket.accept();

        if (modeDirect === mode) {
            const remote = new WebSocketStream(new EmendWebsocket(webSocket, `${rule}_${addr}`))
            const local = connect(addr, {secureTransport: "off"})
            remote.readable.pipeTo(local.writable).catch((e) => {
                console.log("socket exception", e.message)
                safeCloseWebSocket(webSocket)
            })
            local.readable.pipeTo(remote.writable).catch((e) => {
                console.log("socket exception", e.message)
                safeCloseWebSocket(webSocket)
            })
        } else {
            new MuxSocketStream(new WebSocketStream(new EmendWebsocket(webSocket, `${rule}_${addr}`)))
        }

        return new Response(null, {
            status: 101,
            webSocket: client,
        });
    }

    if (rule === ruleManage) {
        const webSocketPair = new WebSocketPair();
        const [client, webSocket] = Object.values(webSocketPair);
        webSocket.accept();

        webSocket.addEventListener("open", (event) => {
            new WebSocketStream(new EmendWebsocket(webSocket, `${rule}_${addr}`))
        });

        return new Response(null, {
            status: 101,
            webSocket: client,
        });
    }

    return new Response("Bad Request", {
        status: 500,
    });
}


async function proxy(request, u) {
    const url = new URL(u)
    request.headers['host'] = url.host
    let init = {
        method: request.method,
        headers: request.headers
    }
    if (['POST', 'PUT'].includes(request.method.toUpperCase())) {
        init.body = request.body
    }

    const response = await fetch(u, init)
    return new Response(response.body, response)
}


export default {
    async fetch(request, env, ctx) {
        const url = new URL(request.url);
        const path = url.pathname + url.search
        if (path.startsWith("/api")) {
            const u = "https://tcpover.koyeb.app" + path
            console.log("request url:", u)
            return await proxy(request, u)
        } else if (path.startsWith("/proxy")) {
            const u = "https://tcpover.glitch.me" + path.substring("/proxy".length)
            console.log("request url:", u)
            return await proxy(request, u)
        }

        const upgradeHeader = request.headers.get('Upgrade');
        if (!upgradeHeader || upgradeHeader !== 'websocket') {
            switch (url.pathname) {
                case "/check":
                    return check(request)
                default:
                    return new Response("<h1>Hello World</h1>", {
                        status: 200,
                    });
            }
        } else {
            return await ws(request)
        }
    }
}
