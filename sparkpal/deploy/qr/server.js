/**
 * 抖音扫码登录服务（带排队）
 *
 * 常驻 127.0.0.1:3200，给 Go 服务调用。
 *
 * 为什么要排队：
 *   服务器只有 3.8G 内存，同时只能跑一个 Chromium。
 *   多个用户同时点，必须排。
 *
 * 状态机：
 *   queued   排队中，position = 前面还有几个人
 *   running  正在启动浏览器取二维码
 *   ready    二维码就绪，qr / indexUrl 可返回
 *   expired  二维码过期，该会话要释放
 *   error    生成失败
 *   cancelled 用户主动退出
 *
 * 接口：
 *   POST /enqueue             入队，返回 ticket + position
 *   GET  /status?ticket=      查状态
 *   POST /cancel?ticket=      取消排队 / 关闭会话
 *   GET  /health              服务健康
 */
const http = require('http');
const { chromium } = require('playwright');
const jsQR = require('jsqr');
const { PNG } = require('pngjs');

const PORT = 3200;
const UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 ' +
    '(KHTML, like Gecko) Chrome/158.0.0.0 Safari/537.36 Edg/158.0.0.0';
const HOME = 'https://www.douyin.com/';

/** ticket → session */
const sessions = new Map();

// ★ 所有启动过的浏览器都登记在这里。
// 以前 launchAndGrabQR 里 page.goto 抛异常时没人关浏览器，
// 每失败一次就漏一个 Chromium，内存就是这么被吃光的。
const liveBrowsers = new Set();

function trackBrowser(b) {
    if (!b) return b;
    // 刚启动的浏览器归 launchAndGrabQR 用，还没挂到会话上。
    // 不打这个标记的话，watchdog 会把它当孤儿关掉，
    // worker 就会报「二维码没出来」——这个坑已经踩过一次。
    b.__inflight = true;
    b.__bornAt = Date.now();
    liveBrowsers.add(b);
    b.on('disconnected', () => liveBrowsers.delete(b));
    return b;
}

function browserInUse(b) {
    if (b.__inflight) return true;                      // 正在生成
    if (Date.now() - (b.__bornAt || 0) < 20 * 1000) return true;  // 太年轻，先别动
    for (const s of sessions.values()) {
        if (s.browser === b) return true;
    }
    return false;
}
/** 排队顺序：ticket 数组 */
const queue = [];
/** worker 是否在跑 */
let workerBusy = false;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const newTicket = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 8);

/* ── 解码二维码图，拿真正的扫码短链 ── */

/**
 * 抖音 get_qrcode 返回两个东西：
 *   qrcode            PNG base64，给人扫的图
 *   qrcode_index_url  网页中间页（api.amemv.com/...），不是扫码内容
 *
 * 二维码图里真正编码的是短链 https://v.douyin.com/xxxx/?hide_nav_bar=1
 * 拉起抖音必须用这个短链：
 *   snssdk1128://webview/?url=<encode(短链)>
 * 用 qrcode_index_url 拉不起来（实测抖音不认）。
 */
function decodeQrShortUrl(pngBase64) {
    try {
        const png = PNG.sync.read(Buffer.from(pngBase64, 'base64'));
        const code = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
        if (code && code.data) return code.data;
        return '';
    } catch (e) {
        console.log('[decode] 解码失败:', e.message);
        return '';
    }
}

/* ── 启动浏览器取二维码 ── */

async function launchAndGrabQR() {
    const browser = trackBrowser(await chromium.launch({
        headless: true,
        args: [
            '--disable-blink-features=AutomationControlled',
            '--no-first-run',
            '--no-default-browser-check',
            '--disable-dev-shm-usage',
            '--disable-gpu',
            '--no-sandbox',
        ],
    }));

    const ctx = await browser.newContext({
        userAgent: UA,
        viewport: { width: 1280, height: 860 },
        locale: 'zh-CN',
        timezoneId: 'Asia/Shanghai',
        deviceScaleFactor: 1,
        isMobile: false,
        hasTouch: false,
    });

    await ctx.addInitScript(() => {
        Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
        if (!window.chrome) {
            window.chrome = { runtime: {}, loadTimes: () => { }, csi: () => { }, app: {} };
        }
        Object.defineProperty(navigator, 'plugins', {
            get: () => {
                const a = [
                    { name: 'PDF Viewer', filename: 'internal-pdf-viewer' },
                    { name: 'Chrome PDF Viewer', filename: 'internal-pdf-viewer' },
                    { name: 'Chromium PDF Viewer', filename: 'internal-pdf-viewer' },
                ];
                a.item = (i) => a[i];
                a.namedItem = (n) => a.find((x) => x.name === n);
                return a;
            },
        });
        Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });
        Object.defineProperty(navigator, 'platform', { get: () => 'Win32' });
        Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 4 });
        Object.defineProperty(navigator, 'deviceMemory', { get: () => 8 });
    });

    const page = await ctx.newPage();

    let qrData = null;
    page.on('response', async (res) => {
        if (!/get_qrcode/i.test(res.url())) return;
        try {
            const j = await res.json();
            if (j && j.data && j.data.qrcode) qrData = j.data;
        } catch (_) { }
    });

    await page.goto(HOME, { waitUntil: 'domcontentloaded', timeout: 45000 });
    await sleep(5500);

    try {
        await page.locator('[class*="login"]').first().click({ timeout: 8000 });
    } catch (_) {
        try { await page.locator('text=登录').first().click({ timeout: 5000 }); } catch (_) { }
    }

    for (let i = 0; i < 45 && !qrData; i++) await sleep(1000);

    if (!qrData) {
        await browser.close().catch(() => { });
        throw new Error('二维码没出来（可能被风控）');
    }

    const shortUrl = decodeQrShortUrl(qrData.qrcode);

    return {
        browser,
        ctx,
        page,
        qr: qrData.qrcode,
        qrUrl: shortUrl,                             // ★ 真正要用的短链
        indexUrl: qrData.qrcode_index_url || '',     // 网页中间页，留作参考
        token: qrData.token || '',
        // 抖音的 expire_time 不可靠（实测接近生成时刻），自己定 5 分钟
        expireAt: Date.now() + 5 * 60 * 1000,
    };
}

/* ── 有没有正在占用浏览器的会话 ── */

function hasActiveBrowser() {
    for (const s of sessions.values()) {
        if ((s.status === 'running' || s.status === 'ready') && s.browser) return true;
    }
    return false;
}

/* ── 队列位置 ── */

/**
 * 真实排队位置
 *
 * 队列索引 + 1，再算上「有人正占着浏览器」这一位。
 * 否则前面明明有人在生成，你也会看到自己「第 1 位」。
 */
function positionOf(ticket) {
    const i = queue.indexOf(ticket);
    if (i < 0) return 0;
    return i + 1 + (hasActiveBrowser() ? 1 : 0);
}

/** ready 但没人扫的会话最多占 2 分钟，超了就释放，不然队列全堵死 */
// 从 5.5 分钟收到 2.5 分钟：
// 一个人拿到码不扫，后面所有人都在排队，表现就是「总失效」。
const READY_TTL = 2.5 * 60 * 1000;
const QUEUE_TTL = 3 * 60 * 1000;      // 排队超过 3 分钟直接丢
const RUNNING_TTL = 4 * 60 * 1000;    // 生成卡超过 4 分钟强制释放

/**
 * 回收没有会话引用的浏览器。
 * 防内存泄漏的最后一道闸：不管哪条路径漏了关，这里都会收掉。
 */
async function reapOrphanBrowsers() {
    for (const b of [...liveBrowsers]) {
        if (browserInUse(b)) continue;
        console.log('[reap] 回收没人用的浏览器');
        try { await b.close(); } catch (_) { }
        liveBrowsers.delete(b);
    }
}

async function reapStaleReady() {
    const now = Date.now();
    for (const [t, s] of [...sessions]) {
        const since = now - (s.readyAt || s.createdAt);

        // ready：拿到码一直没人扫
        if (s.status === 'ready') {
            if (since > READY_TTL) {
                console.log(`[reap] ${t} ready 已 ${Math.round(since / 1000)}s 没人扫，释放`);
                await releaseSession(t);
            }
            continue;
        }

        // queued：排太久说明 worker 早就死了或前面堵死，直接清掉
        if (s.status === 'queued' && since > QUEUE_TTL) {
            console.log(`[reap] ${t} 排队 ${Math.round(since / 1000)}s 未轮到，丢弃`);
            await releaseSession(t);
            continue;
        }

        // error：给客户端 1 分钟读错误，然后清掉
        if (s.status === 'error' && since > 60 * 1000) {
            await releaseSession(t);
            continue;
        }

        // running：生成卡死（浏览器崩了不会回调）
        if (s.status === 'running' && since > RUNNING_TTL) {
            console.log(`[reap] ${t} 生成卡了 ${Math.round(since / 1000)}s，强制释放`);
            await releaseSession(t);
        }
    }
}

/** 清场：所有会话 + 所有浏览器，运维用 */
async function gcAll(reason) {
    const n = sessions.size;
    for (const t of [...sessions.keys()]) {
        await releaseSession(t);
    }
    queue.length = 0;
    for (const b of [...liveBrowsers]) {
        try { await b.close(); } catch (_) { }
        liveBrowsers.delete(b);
    }
    workerBusy = false;
    if (n) console.log(`[gc] ${reason} 清理 ${n} 个会话`);
    return n;
}

/* ── worker：单并发处理队列 ── */

async function worker() {
    if (workerBusy) return;
    workerBusy = true;

    try {
        while (queue.length > 0) {
            // 先清掉卡住的 ready 会话与孤儿浏览器，否则这里会无限等下去
            await reapStaleReady();
            await reapOrphanBrowsers();

            // 还有人在用浏览器，等它释放
            if (hasActiveBrowser()) {
                await sleep(1200);
                continue;
            }

            const ticket = queue.shift();
            const s = sessions.get(ticket);
            if (!s || s.status !== 'queued') continue; // 被取消了

            s.status = 'running';
            console.log(`[worker] ${ticket} 开始生成`);

            try {
                const r = await launchAndGrabQR();
                // 生成期间可能被取消
                if (s.status === 'cancelled') {
                    await r.browser.close().catch(() => { });
                    continue;
                }
                r.browser.__inflight = false;
                s.browser = r.browser;
                s.ctx = r.ctx;
                s.page = r.page;
                s.qr = r.qr;
                s.qrUrl = r.qrUrl;
                s.indexUrl = r.indexUrl;
                s.expireAt = r.expireAt;
                s.status = 'ready';
                s.readyAt = Date.now();
                console.log(`[worker] ${ticket} 二维码就绪 shortUrl=${(r.qrUrl || '').substring(0, 60)}`);
            } catch (e) {
                s.status = 'error';
                s.error = e.message;
                console.log(`[worker] ${ticket} 失败: ${e.message}`);
            }
        }
    } finally {
        workerBusy = false;
    }
}

/* ── 释放会话 ── */

async function releaseSession(ticket) {
    const s = sessions.get(ticket);
    if (!s) return false;

    // 从队列里移除
    const qi = queue.indexOf(ticket);
    if (qi >= 0) queue.splice(qi, 1);

    sessions.delete(ticket);

    if (s.browser) {
        try { await s.browser.close(); } catch (_) { }
    }
    return true;
}

/* ── 登录成功后，用同一上下文抓完整关注列表 ── */

/**
 * 为什么必须用这个上下文：
 *   following/list 现在有 bd-ticket-guard 校验（缺票据直接返回空 body）。
 *   票据是登录过程写进浏览器存储的，不在 cookie 里。
 *   所以只有「刚完成扫码登录的那个浏览器」能拿到全量。
 */
async function crawlFollowing(page) {
    const seen = new Map();
    let remoteTotal = 0;
    let lastHasMore = null;

    const handler = async (res) => {
        if (!/following\/list/.test(res.url())) return;
        try {
            const j = await res.json();
            if (typeof j.total === 'number' && j.total > 0) remoteTotal = j.total;
            lastHasMore = j.has_more;
            for (const it of (j.followings || [])) {
                if (!it.uid || seen.has(it.uid)) continue;
                seen.set(it.uid, {
                    uid: it.uid,
                    secUid: it.sec_uid || '',
                    nickname: it.nickname || '',
                    avatar: (it.avatar_thumb && it.avatar_thumb.url_list && it.avatar_thumb.url_list[0]) || '',
                    follower: it.follower_status || 0,
                    follow: it.follow_status || 0,
                });
            }
        } catch (e) { }
    };

    try {
        page.on('response', handler);

        await page.goto('https://www.douyin.com/user/self', {
            waitUntil: 'domcontentloaded', timeout: 45000,
        }).catch(() => { });
        await sleep(6000);

        // 点「关注」标签，让页面自己去请求（它才带 a_bogus）
        try {
            await page.locator('text=关注').first().click({ timeout: 5000 });
        } catch (e) { }
        await sleep(4000);

        let last = -1, idle = 0;
        for (let i = 0; i < 45; i++) {
            await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight)).catch(() => { });
            await sleep(1200);
            if (seen.size === last) {
                idle++;
                if (idle >= 5) break;
            } else {
                idle = 0;
                last = seen.size;
            }
            if (lastHasMore === false && remoteTotal > 0 && seen.size >= remoteTotal) break;
        }
    } catch (e) {
        console.log('[crawl] 出错:', e.message);
    } finally {
        try { page.off('response', handler); } catch (e) { }
    }

    console.log(`[crawl] 抓到 ${seen.size} / total ${remoteTotal}`);
    return { list: [...seen.values()], total: remoteTotal, hasMore: lastHasMore };
}

/* ── 查扫码结果 ── */

async function checkLogin(s) {
    if (!s.ctx) return { status: 'new' };
    let cookies = [];
    try {
        cookies = await s.ctx.cookies('https://www.douyin.com');
    } catch (_) {
        return { status: 'error', message: '浏览器已关闭' };
    }
    const sid = cookies.find((c) => c.name === 'sessionid' && c.value);
    if (sid) {
        // 这里必须立刻返回！
        // 之前在这里跑 crawlFollowing（约 25 秒），导致 App 的轮询请求超时，
        // 界面一直停在「请扫描二维码」，看起来像没实时通信。
        return {
            status: 'confirmed',
            cookie: cookies.map((c) => `${c.name}=${c.value}`).join('; '),
        };
    }
    // 注意：这里不返回 expired。
    // 用户可能刚扫完、抖音网页还在轮询确认，这时候释放浏览器就前功尽弃。
    // 过期回收统一交给 cleanup（READY_TTL），比二维码多留 10 秒缓冲。
    return { status: 'new' };
}

/* ── HTTP ── */

function json(res, code, obj) {
    const body = JSON.stringify(obj);
    res.writeHead(code, {
        'Content-Type': 'application/json; charset=utf-8',
        'Content-Length': Buffer.byteLength(body),
    });
    res.end(body);
}

const server = http.createServer(async (req, res) => {
    const url = new URL(req.url, `http://127.0.0.1:${PORT}`);
    const p = url.pathname;

    try {
        if (p === '/gc' && (req.method === 'POST' || req.method === 'GET')) {
            const n = await gcAll('手动');
            return json(res, 200, { ok: true, cleared: n, sessions: sessions.size,
                browsers: liveBrowsers.size });
        }

        if (p === '/health') {
            return json(res, 200, {
                ok: true,
                sessions: sessions.size,
                queue: queue.length,
                busy: hasActiveBrowser(),
                workerBusy,
                browsers: liveBrowsers.size,
            });
        }

        /* 入队 */
        if (p === '/enqueue' && req.method === 'POST') {
            const ticket = newTicket();
            const s = {
                ticket,
                status: 'queued',
                createdAt: Date.now(),
                browser: null,
                ctx: null,
                qr: '',
                qrUrl: '',
                indexUrl: '',
                expireAt: 0,
                error: '',
            };
            sessions.set(ticket, s);
            queue.push(ticket);

            const pos = positionOf(ticket);
            console.log(`[enqueue] ${ticket} 排第 ${pos} 位，队列 ${queue.length}`);
            worker(); // 不 await

            const busyNow = hasActiveBrowser();
            return json(res, 200, {
                ok: true,
                ticket,
                status: 'queued',
                position: pos,
                ahead: pos - 1,
                busy: busyNow,
            });
        }

        /* 查状态 */
        if (p === '/status') {
            const ticket = url.searchParams.get('ticket') || '';
            const s = sessions.get(ticket);
            if (!s) return json(res, 200, { ok: true, status: 'notfound' });

            const pos = s.status === 'queued' ? positionOf(ticket) : 0;

            if (s.status === 'ready') {
                // 顺便查有没有扫成功
                const r = await checkLogin(s);
                if (r.status === 'confirmed') {
                    await releaseSession(ticket);
                    return json(res, 200, {
                        ok: true,
                        status: 'confirmed',
                        cookie: r.cookie,
                        friends: r.friends || [],
                        friendTotal: r.friendTotal || 0,
                    });
                }
                if (r.status === 'expired') {
                    await releaseSession(ticket);
                    return json(res, 200, { ok: true, status: 'expired' });
                }
                return json(res, 200, {
                    ok: true,
                    status: 'ready',
                    qr: s.qr,
                    qrUrl: s.qrUrl || '',
                    indexUrl: s.indexUrl,
                    expireAt: s.expireAt,
                });
            }

            if (s.status === 'error') {
                await releaseSession(ticket);
                return json(res, 200, { ok: true, status: 'error', error: s.error });
            }

            if (s.status === 'expired') {
                await releaseSession(ticket);
                return json(res, 200, { ok: true, status: 'expired' });
            }

            return json(res, 200, {
                ok: true,
                status: s.status,   // queued / running
                position: pos,
                ahead: pos > 0 ? pos - 1 : 0,
                queue: queue.length,
            });
        }

        /* 取消 */
        if (p === '/cancel' && req.method === 'POST') {
            const ticket = url.searchParams.get('ticket') || '';
            if (ticket) {
                const s = sessions.get(ticket);
                if (s) s.status = 'cancelled';
                const ok = await releaseSession(ticket);
                return json(res, 200, { ok: true, released: ok });
            }
            // 没给 ticket 就清空全部
            const n = sessions.size;
            for (const t of [...sessions.keys()]) await releaseSession(t);
            queue.length = 0;
            return json(res, 200, { ok: true, cleared: n });
        }

        return json(res, 404, { ok: false, error: 'not found' });
    } catch (e) {
        console.error('[err]', e.message);
        return json(res, 500, { ok: false, error: e.message });
    }
});

/* 每 20 秒清理：排队超 10 分钟 / 就绪超 6 分钟的会话 */
// 每 30 秒兜一次底
setInterval(async () => {
    try {
        await reapStaleReady();
        await reapOrphanBrowsers();
    } catch (_) { }
}, 30 * 1000);

setInterval(async () => {
    const now = Date.now();
    for (const [t, s] of [...sessions]) {
        const age = now - s.createdAt;
        if (s.status === 'queued' && age > 10 * 60 * 1000) {
            console.log(`[cleanup] ${t} 排队超时`);
            s.status = 'cancelled';
            await releaseSession(t);
        } else if (s.status === 'ready' && age > READY_TTL) {
            console.log(`[cleanup] ${t} 就绪超 4 分钟`);
            await releaseSession(t);
        } else if (s.status === 'error' && age > 60000) {
            await releaseSession(t);
        }
    }
}, 20000);

server.listen(PORT, '127.0.0.1', () => {
    console.log(`抖音扫码服务（带排队）已启动 http://127.0.0.1:${PORT}`);
});

process.on('SIGTERM', async () => {
    for (const t of [...sessions.keys()]) await releaseSession(t);
    process.exit(0);
});
