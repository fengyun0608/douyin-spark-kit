/**
 * 抖音二维码登录服务
 *
 * 为什么需要浏览器：
 *   get_qrcode 是 sso/网页接口，纯 HTTP 请求会被 uc-secure-tool-detect 拦，
 *   返回的二维码永远处于 loading（接口根本不发出去）。
 *   Playwright 把自动化特征处理得比较干净，实测能稳定拿到二维码。
 *
 * 核心发现：
 *   响应里的 data.qrcode 就是 PNG 的 base64，直接显示即可，不用截图。
 *   但「拉起抖音」要用的不是 qrcode_index_url，而是二维码图里编码的
 *   https://v.douyin.com/xxx/?hide_nav_bar=1 短链（见 decode-qr.js）。
 *
 * 用法：
 *   npm i playwright jsqr pngjs
 *   node src/qr-login.js
 *   curl -X POST http://127.0.0.1:3200/start
 */
const http = require('http');
const { chromium } = require('playwright');

const PORT = Number(process.env.PORT || 3200);
const UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 ' +
  '(KHTML, like Gecko) Chrome/158.0.0.0 Safari/537.36 Edg/158.0.0.0';
const HOME = 'https://www.douyin.com/';

/** 二维码就绪后最多留 4 分钟（抖音二维码本身 5 分钟过期） */
const READY_TTL = 4 * 60 * 1000;

/** ticket → { status, browser, ctx, page, qr, shortUrl, expireAt, createdAt } */
const sessions = new Map();

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const newTicket = () =>
  Date.now().toString(36) + Math.random().toString(36).slice(2, 8);

/* ── 反检测：页面脚本执行前注入 ── */

const STEALTH = () => {
  try {
    Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
    if (!window.chrome) {
      window.chrome = { runtime: {}, loadTimes: () => {}, csi: () => {}, app: {} };
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
  } catch (e) {}
};

/* ── 起一个会话，取二维码 ── */

async function launchAndGrabQR() {
  const browser = await chromium.launch({
    headless: true,
    args: [
      '--disable-blink-features=AutomationControlled',
      '--no-first-run',
      '--no-default-browser-check',
      '--disable-dev-shm-usage',
      '--disable-gpu',
      '--no-sandbox',
    ],
  });

  const ctx = await browser.newContext({
    userAgent: UA,
    viewport: { width: 1280, height: 860 },
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    deviceScaleFactor: 1,
    isMobile: false,
    hasTouch: false,
  });
  await ctx.addInitScript(STEALTH);

  const page = await ctx.newPage();

  let qrData = null;
  page.on('response', async (res) => {
    if (!/get_qrcode/i.test(res.url())) return;
    try {
      const j = await res.json();
      if (j && j.data && j.data.qrcode) qrData = j.data;
    } catch (e) {}
  });

  await page.goto(HOME, { waitUntil: 'domcontentloaded', timeout: 45000 });
  await sleep(5500);

  // 点开登录弹窗
  try {
    await page.locator('[class*="login"]').first().click({ timeout: 8000 });
  } catch (e) {
    try {
      await page.locator('text=登录').first().click({ timeout: 5000 });
    } catch (e2) {}
  }

  for (let i = 0; i < 45 && !qrData; i++) await sleep(1000);

  if (!qrData) {
    await browser.close().catch(() => {});
    throw new Error('二维码没出来（可能被风控）');
  }

  return {
    browser,
    ctx,
    page,
    qr: qrData.qrcode, // PNG base64
    indexUrl: qrData.qrcode_index_url || '',
    expireAt: qrData.expire_time ? qrData.expire_time * 1000 : Date.now() + 300000,
  };
}

/* ── 查登录态 ── */

async function checkLogin(s) {
  let cookies = [];
  try {
    cookies = await s.ctx.cookies('https://www.douyin.com');
  } catch (e) {
    return { status: 'error', message: '浏览器已关闭' };
  }

  const sid = cookies.find((c) => c.name === 'sessionid' && c.value);
  if (sid) {
    return {
      status: 'confirmed',
      cookie: cookies.map((c) => `${c.name}=${c.value}`).join('; '),
    };
  }
  if (Date.now() > s.expireAt) return { status: 'expired' };
  return { status: 'new' };
}

async function release(ticket) {
  const s = sessions.get(ticket);
  if (!s) return false;
  sessions.delete(ticket);
  if (s.browser) await s.browser.close().catch(() => {});
  return true;
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
    if (p === '/health') {
      return json(res, 200, { ok: true, sessions: sessions.size });
    }

    if (p === '/start' && req.method === 'POST') {
      const t0 = Date.now();
      let s;
      try {
        s = await launchAndGrabQR();
      } catch (e) {
        return json(res, 500, { ok: false, error: e.message });
      }
      const ticket = newTicket();
      sessions.set(ticket, { ...s, status: 'ready', createdAt: Date.now() });
      console.log(`[start] ${ticket} 耗时 ${Date.now() - t0}ms`);
      return json(res, 200, {
        ok: true,
        ticket,
        qr: s.qr,
        indexUrl: s.indexUrl,
        expireAt: s.expireAt,
      });
    }

    if (p === '/status') {
      const ticket = url.searchParams.get('ticket') || '';
      const s = sessions.get(ticket);
      if (!s) return json(res, 200, { ok: true, status: 'notfound' });

      const r = await checkLogin(s);
      if (r.status === 'confirmed') {
        await release(ticket);
        return json(res, 200, { ok: true, ...r });
      }
      if (r.status === 'expired' || r.status === 'error') {
        await release(ticket);
        return json(res, 200, { ok: true, ...r });
      }
      return json(res, 200, { ok: true, status: 'ready' });
    }

    if (p === '/close' && req.method === 'POST') {
      return json(res, 200, { ok: true, released: await release(url.searchParams.get('ticket') || '') });
    }

    return json(res, 404, { ok: false, error: 'not found' });
  } catch (e) {
    console.error('[err]', e.message);
    return json(res, 500, { ok: false, error: e.message });
  }
});

// 清理超时会话
setInterval(async () => {
  const now = Date.now();
  for (const [t, s] of [...sessions]) {
    if (now - s.createdAt > READY_TTL) {
      console.log(`[cleanup] ${t}`);
      await release(t);
    }
  }
}, 20000);

server.listen(PORT, '127.0.0.1', () => {
  console.log(`douyin qr-login 服务: http://127.0.0.1:${PORT}`);
});

process.on('SIGTERM', async () => {
  for (const t of [...sessions.keys()]) await release(t);
  process.exit(0);
});
