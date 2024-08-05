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

let uuid = null;
const mutex = new Lock()

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
    const uid = url.searchParams.get("uid")
    const rule = url.searchParams.get("rule")

    const regex = /^([a-zA-Z0-9.]+):(\d+)$/
    if (rule === "Connector" && regex.test(uid)) {
        const tokens = regex.exec(uid)
        const hostname = tokens[1]
        const port = parseInt(tokens[2])
        console.log(`${uid} hostname: ${hostname}, port:${port}`)

        const webSocketPair = new WebSocketPair();
        const [client, webSocket] = Object.values(webSocketPair);
        webSocket.accept();

        const remote = new WebSocketStream(new EmendWebsocket(webSocket, `${rule}_${uid}`))
        const local = connect(uid, { secureTransport: "off" })
        remote.readable.pipeTo(local.writable).catch((e) => {
            console.log("socket exception", e.message)
            safeCloseWebSocket(webSocket)
        })
        local.readable.pipeTo(remote.writable).catch((e) => {
            console.log("socket exception", e.message)
            safeCloseWebSocket(webSocket)
        })
        
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
                    return await proxy(request, "https://www.bing.com"+path)
            }
        } else {
            return await ws(request)
        }
    }
}
