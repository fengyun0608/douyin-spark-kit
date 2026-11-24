/**
 * 从二维码图里解出真正的登录短链
 *
 * 为什么需要这一步：
 *   get_qrcode 返回两个地址，很容易搞混：
 *     qrcode_index_url  https://api.amemv.com/ucenter_web/...token=xxx   （网页中间页）
 *     二维码图里的内容   https://v.douyin.com/xxxx/?hide_nav_bar=1        （真正的扫码内容）
 *   拉起抖音必须用后者，用前者必然失败（抖音不认）。
 *
 * 用法：
 *   const { decodeQrShortUrl } = require('./decode-qr');
 *   const shortUrl = decodeQrShortUrl(pngBase64);
 */
const jsQR = require('jsqr');
const { PNG } = require('pngjs');

/**
 * @param {string} pngBase64  data.qrcode（不带 data:image 前缀）
 * @returns {string}          形如 https://v.douyin.com/xxx/?hide_nav_bar=1，失败返回空串
 */
function decodeQrShortUrl(pngBase64) {
  try {
    const png = PNG.sync.read(Buffer.from(pngBase64, 'base64'));
    const code = jsQR(new Uint8ClampedArray(png.data), png.width, png.height);
    return code && code.data ? code.data : '';
  } catch (e) {
    return '';
  }
}

module.exports = { decodeQrShortUrl };

/* ── 直接跑：node src/decode-qr.js <图片路径> ── */
if (require.main === module) {
  const fs = require('fs');
  const file = process.argv[2];
  if (!file) {
    console.log('用法: node src/decode-qr.js qr.png');
    process.exit(1);
  }
  const b64 = fs.readFileSync(file).toString('base64');
  const url = decodeQrShortUrl(b64);
  if (url) {
    console.log('内容:', url);
    console.log('拉起用:', `snssdk1128://webview/?url=${encodeURIComponent(url)}`);
  } else {
    console.log('解码失败（图里可能不是二维码）');
  }
}
