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

const manageSocket = {}
const groupSocket = {}
let uuid = null;
const mutex = new Lock()

function check(request) {
    if (!uuid) {
        uuid = new Date();
    }
    return new Response("code is:" + uuid.valueOf())
}

async function ws(request) {
    const url = new URL(request.url);
    const uid = url.searchParams.get("uid")
    const code = url.searchParams.get("code")
    const rule = url.searchParams.get("rule")
    let targetConn
    if (rule === "Connector") {
        targetConn = manageSocket[uid]
        if (!targetConn) {
            console.log("the target Agent websocket not exist", uid)
            return new Response("agent not running.");
        }
    }
    console.log(`uid: ${uid}, code: ${code}, rule: ${rule}`)
    const webSocketPair = new WebSocketPair();
    const [client, webSocket] = Object.values(webSocketPair);
    webSocket.accept();

    if (rule === "manage") {
        webSocket.addEventListener("open", () => {
            manageSocket[uid] = new EmendWebsocket(webSocket, `${rule}_${uid}`)
        })
        webSocket.addEventListener("error", (e) => {
            console.log("socket onerror", e.message);
            manageSocket[uid] = null
        })
        webSocket.addEventListener("close", () => {
            console.log("socket closed");
            manageSocket[uid] = null
        })

        return new Response(null, {
            status: 101,
            webSocket: client,
        });
    }

    await mutex.acquire()
    if (groupSocket[code]) {
        webSocket.addEventListener("open", async () => {
            groupSocket[code].second = new WebSocketStream(new EmendWebsocket(socket, `${rule}_${code}_${uid}`))
            const connPair = groupSocket[code]
            await mutex.release()

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
        })
    } else {
        webSocket.addEventListener("open", async () => {
            groupSocket[code] = {
                first: new WebSocketStream(new EmendWebsocket(webSocket, `${rule}_${code}_${uid}`)),
            }
            await mutex.release()
        })
        const data = JSON.stringify({
            Command: 0x01,
            Data: {Code: code}
        })
        console.log("send data:", data)
        targetConn.send(data)
    }


    return new Response(null, {
        status: 101,
        webSocket: client,
    });
}


export default {
    async fetch(request, env, ctx) {
        const upgradeHeader = request.headers.get('Upgrade');
        if (!upgradeHeader || upgradeHeader !== 'websocket') {
            const url = new URL(request.url);
            switch (url.pathname) {
                case "/check":
                    return check(request)
                default:
                    return new Response("invalid request")
            }
        } else {
            return await ws(request)
        }
    }
}
