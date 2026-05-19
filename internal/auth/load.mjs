// Extract cookies for ONE target URL from the local Chrome profile, decrypted
// via the OS keychain (chrome-cookies-secure). Input JSON on stdin:
//   {target_url, chrome_profile, timeout_millis}
// Output JSON to $BOURSOBANK_OUTPUT_PATH: {cookie_header, cookie_count, error}
// The Go side calls this twice (clients.boursobank.com + clients.boursorama.com)
// and concatenates — the dual-domain requirement. Copies the Cookies DB to a
// temp dir first so it works even while Chrome is open (no lock).
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

function out() { return process.env.BOURSOBANK_OUTPUT_PATH || ''; }
async function write(o) { const p = out(); if (!p) throw new Error('BOURSOBANK_OUTPUT_PATH missing'); await fs.writeFile(p, JSON.stringify(o), 'utf8'); }

async function profileRoot() {
  const h = os.homedir();
  const c = process.platform === 'darwin'
    ? [path.join(h, 'Library', 'Application Support', 'Google', 'Chrome')]
    : process.platform === 'linux'
      ? [path.join(h, '.config', 'google-chrome'), path.join(h, '.config', 'chromium')]
      : [path.join(process.env.LOCALAPPDATA || path.join(h, 'AppData', 'Local'), 'Google', 'Chrome', 'User Data')];
  for (const d of c) { try { await fs.stat(d); return d; } catch {} }
  return c[0];
}

async function resolveCookieFile(profile) {
  if (profile && (profile.includes('/') || profile.includes('\\'))) return ensure(profile);
  const name = profile && profile.trim() ? profile.trim() : 'Default';
  return ensure(path.join(await profileRoot(), name));
}
async function ensure(p) {
  const st = await fs.stat(p).catch(() => null);
  if (!st) throw new Error('Chrome profile not found: ' + p);
  if (st.isDirectory()) {
    for (const f of [path.join(p, 'Network', 'Cookies'), path.join(p, 'Cookies')]) {
      try { if ((await fs.stat(f)).isFile()) return f; } catch {}
    }
    throw new Error('No Cookies DB under ' + p);
  }
  return p;
}

async function readStdin() {
  return new Promise((res, rej) => { let d = ''; process.stdin.setEncoding('utf8'); process.stdin.on('data', c => d += c); process.stdin.on('end', () => res(d)); process.stdin.on('error', rej); });
}

async function main() {
  const inp = JSON.parse((await readStdin()) || '{}');
  const targetUrl = String(inp.target_url || '').trim();
  if (!targetUrl) throw new Error('target_url missing');
  const timeoutMs = Number.isFinite(inp.timeout_millis) && inp.timeout_millis > 0 ? inp.timeout_millis : 5000;
  const cookieFile = await resolveCookieFile(inp.chrome_profile ? String(inp.chrome_profile) : '');
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'bb-ck-'));
  try { await fs.copyFile(cookieFile, path.join(dir, 'Cookies')); } catch { /* fall back to original dir */ }
  const cookiesDir = await fs.stat(path.join(dir, 'Cookies')).then(() => dir).catch(() => path.dirname(cookieFile));
  const mod = await import('chrome-cookies-secure');
  const cc = mod?.default ?? mod;
  const timed = (p) => Promise.race([p, new Promise((_, r) => setTimeout(() => r(new Error('chrome cookie read timeout')), timeoutMs))]);
  const cookies = await timed(cc.getCookiesPromised(targetUrl, 'puppeteer', cookiesDir));
  const seen = new Set(); const pairs = [];
  for (const c of (Array.isArray(cookies) ? cookies : [])) {
    const n = c?.name ? String(c.name) : ''; const v = c?.value ? String(c.value) : '';
    if (!n || !v || seen.has(n)) continue;        // capture-all: no name whitelist
    seen.add(n); pairs.push(n + '=' + v);
  }
  await write({ cookie_header: pairs.join('; '), cookie_count: pairs.length, error: '' });
}

main().catch(async (e) => { try { await write({ cookie_header: '', cookie_count: 0, error: String(e && e.message || e) }); } catch {} process.exitCode = 1; });
