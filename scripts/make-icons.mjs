#!/usr/bin/env node
// Dependency-free PNG icon generator.
// Decodes an RGBA/RGB PNG, box-resizes it, optionally composites it over a
// solid background, and re-encodes RGBA PNG. Used to derive favicon / PWA /
// apple-touch icons from the single Lab AP source logo.
//
//   node scripts/make-icons.mjs --out apps/portal/static \
//     --source path/to/logo.png --wordmark path/to/logo_new.png

import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { pathToFileURL } from 'node:url';

const CRC_TABLE = (() => {
  const table = new Int32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c;
  }
  return table;
})();

function crc32(buf) {
  let c = -1;
  for (let i = 0; i < buf.length; i++)
    c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  return (c ^ -1) >>> 0;
}

function chunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type, 'ascii'), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(body));
  return Buffer.concat([len, body, crc]);
}

export function decodePng(file) {
  const buf = fs.readFileSync(file);
  let p = 8;
  let width = 0;
  let height = 0;
  let bitDepth = 0;
  let colorType = 0;
  const idat = [];
  while (p < buf.length) {
    const len = buf.readUInt32BE(p);
    const type = buf.toString('ascii', p + 4, p + 8);
    const data = buf.subarray(p + 8, p + 8 + len);
    if (type === 'IHDR') {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      bitDepth = data[8];
      colorType = data[9];
    } else if (type === 'IDAT') idat.push(data);
    else if (type === 'IEND') break;
    p += 12 + len;
  }
  if (bitDepth !== 8 || (colorType !== 6 && colorType !== 2)) {
    throw new Error(
      `${file}: unsupported PNG (bitDepth=${bitDepth} colorType=${colorType})`
    );
  }
  const ch = colorType === 6 ? 4 : 3;
  const raw = zlib.inflateSync(Buffer.concat(idat));
  const stride = width * ch;
  const out = Buffer.alloc(height * stride);
  let pos = 0;
  for (let y = 0; y < height; y++) {
    const filter = raw[pos++];
    const line = raw.subarray(pos, pos + stride);
    pos += stride;
    const cur = out.subarray(y * stride, (y + 1) * stride);
    const prev = y > 0 ? out.subarray((y - 1) * stride, y * stride) : null;
    for (let x = 0; x < stride; x++) {
      const a = x >= ch ? cur[x - ch] : 0;
      const b = prev ? prev[x] : 0;
      const c = prev && x >= ch ? prev[x - ch] : 0;
      let v = line[x];
      if (filter === 1) v += a;
      else if (filter === 2) v += b;
      else if (filter === 3) v += (a + b) >> 1;
      else if (filter === 4) {
        const pa = Math.abs(b - c);
        const pb = Math.abs(a - c);
        const pc = Math.abs(a + b - 2 * c);
        v += pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
      }
      cur[x] = v & 0xff;
    }
  }
  // normalise to RGBA
  const rgba = Buffer.alloc(width * height * 4);
  for (let i = 0; i < width * height; i++) {
    rgba[i * 4] = out[i * ch];
    rgba[i * 4 + 1] = out[i * ch + 1];
    rgba[i * 4 + 2] = out[i * ch + 2];
    rgba[i * 4 + 3] = ch === 4 ? out[i * ch + 3] : 255;
  }
  return { width, height, data: rgba };
}

export function encodePng({ width, height, data }) {
  const stride = width * 4;
  const raw = Buffer.alloc(height * (stride + 1));
  for (let y = 0; y < height; y++) {
    raw[y * (stride + 1)] = 0; // filter: none
    data.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 6; // RGBA
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw, { level: 9 })),
    chunk('IEND', Buffer.alloc(0))
  ]);
}

/** Area-average (box) downscale — correct and clean for shrinking logos. */
export function resize({ width, height, data }, tw, th) {
  const out = Buffer.alloc(tw * th * 4);
  const xRatio = width / tw;
  const yRatio = height / th;
  for (let ty = 0; ty < th; ty++) {
    const y0 = Math.floor(ty * yRatio);
    const y1 = Math.min(
      height,
      Math.max(y0 + 1, Math.floor((ty + 1) * yRatio))
    );
    for (let tx = 0; tx < tw; tx++) {
      const x0 = Math.floor(tx * xRatio);
      const x1 = Math.min(
        width,
        Math.max(x0 + 1, Math.floor((tx + 1) * xRatio))
      );
      let r = 0;
      let g = 0;
      let b = 0;
      let a = 0;
      let n = 0;
      for (let y = y0; y < y1; y++) {
        for (let x = x0; x < x1; x++) {
          const i = (y * width + x) * 4;
          const alpha = data[i + 3];
          // premultiply so transparent pixels do not darken edges
          r += data[i] * alpha;
          g += data[i + 1] * alpha;
          b += data[i + 2] * alpha;
          a += alpha;
          n++;
        }
      }
      const o = (ty * tw + tx) * 4;
      if (a > 0) {
        out[o] = Math.round(r / a);
        out[o + 1] = Math.round(g / a);
        out[o + 2] = Math.round(b / a);
      }
      out[o + 3] = Math.round(a / n);
    }
  }
  return { width: tw, height: th, data: out };
}

/** Composite over a solid background colour (hex). */
export function flatten({ width, height, data }, hex) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  const out = Buffer.alloc(width * height * 4);
  for (let i = 0; i < width * height; i++) {
    const a = data[i * 4 + 3] / 255;
    out[i * 4] = Math.round(data[i * 4] * a + r * (1 - a));
    out[i * 4 + 1] = Math.round(data[i * 4 + 1] * a + g * (1 - a));
    out[i * 4 + 2] = Math.round(data[i * 4 + 2] * a + b * (1 - a));
    out[i * 4 + 3] = 255;
  }
  return { width, height, data: out };
}

/** Trim fully transparent margins so the mark fills the icon box. */
export function trim(img) {
  const { width, height, data } = img;
  let minX = width;
  let minY = height;
  let maxX = 0;
  let maxY = 0;
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      if (data[(y * width + x) * 4 + 3] > 8) {
        if (x < minX) minX = x;
        if (y < minY) minY = y;
        if (x > maxX) maxX = x;
        if (y > maxY) maxY = y;
      }
    }
  }
  if (minX > maxX) return img;
  const tw = maxX - minX + 1;
  const th = maxY - minY + 1;
  const out = Buffer.alloc(tw * th * 4);
  for (let y = 0; y < th; y++) {
    data.copy(
      out,
      y * tw * 4,
      ((y + minY) * width + minX) * 4,
      ((y + minY) * width + maxX + 1) * 4
    );
  }
  return { width: tw, height: th, data: out };
}

/** Pad into a square canvas with transparent margin (keeps aspect ratio). */
export function square(img, pad = 0.1) {
  const side = Math.round(Math.max(img.width, img.height) * (1 + pad * 2));
  const out = Buffer.alloc(side * side * 4);
  const ox = Math.round((side - img.width) / 2);
  const oy = Math.round((side - img.height) / 2);
  for (let y = 0; y < img.height; y++) {
    img.data.copy(
      out,
      ((y + oy) * side + ox) * 4,
      y * img.width * 4,
      (y + 1) * img.width * 4
    );
  }
  return { width: side, height: side, data: out };
}

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i > -1 ? process.argv[i + 1] : fallback;
}

function write(dir, name, img) {
  const file = path.join(dir, name);
  fs.writeFileSync(file, encodePng(img));
  const kb = (fs.statSync(file).size / 1024).toFixed(1);
  console.log(
    `  ${path.relative(process.cwd(), file).replaceAll('\\', '/')}  ${img.width}x${img.height}  ${kb} KB`
  );
}

function main() {
  const outDir = arg('out');
  const source = arg('source');
  const wordmark = arg('wordmark');
  if (!outDir || !source) {
    console.error(
      'usage: node scripts/make-icons.mjs --out <dir> --source <logo.png> [--wordmark <file.png>]'
    );
    process.exit(1);
  }
  fs.mkdirSync(outDir, { recursive: true });
  const CREAM = '#fdfbf7'; // --ah-cream
  console.log(`icons -> ${outDir}`);
  const mark = square(trim(decodePng(source)), 0.06);
  const raw = trim(decodePng(source));
  write(outDir, 'favicon-32.png', resize(mark, 32, 32));
  write(outDir, 'icon-192.png', resize(flatten(mark, CREAM), 192, 192));
  write(outDir, 'icon-512.png', resize(flatten(mark, CREAM), 512, 512));
  write(outDir, 'apple-touch-icon.png', resize(flatten(mark, CREAM), 180, 180));
  // transparent, un-padded mark for in-app navbars (keeps the real aspect)
  const markH = 96;
  write(
    outDir,
    'logo-mark.png',
    resize(raw, Math.round((raw.width / raw.height) * markH), markH)
  );
  if (wordmark && fs.existsSync(wordmark)) {
    const wm = trim(decodePng(wordmark));
    const targetW = 480;
    write(
      outDir,
      'logo-lab.png',
      resize(wm, targetW, Math.round((wm.height / wm.width) * targetW))
    );
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)
  main();
