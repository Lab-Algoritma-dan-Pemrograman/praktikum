import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import {
  runtimeFrameURL,
  boundedPreview,
  normalizeBrowserTestOutput
} from '../src/browser-preview.ts';

test('preview accepts only clean HTTPS or local runtime origins', () => {
  assert.equal(
    runtimeFrameURL('https://staging.algohub.web.id'),
    'https://staging.algohub.web.id/runtimes/v3/frame.html'
  );
  assert.equal(
    runtimeFrameURL('http://portal.localhost:5173'),
    'http://portal.localhost:5173/runtimes/v3/frame.html'
  );
  for (const origin of [
    'javascript:alert(1)',
    'data:text/html,x',
    'http://example.com',
    'https://user:password@example.com',
    'https://example.com/path',
    'https://example.com?token=secret',
    'https://example.com#fragment'
  ])
    assert.throws(() => runtimeFrameURL(origin));
});

test('browser testcase output normalizes only line endings and terminal newlines', () => {
  assert.equal(normalizeBrowserTestOutput('answer\r\n'), 'answer');
  assert.equal(normalizeBrowserTestOutput('answer\n\n'), 'answer');
  assert.equal(normalizeBrowserTestOutput('answer '), 'answer ');
  assert.equal(normalizeBrowserTestOutput(' before\n'), ' before');
  assert.equal(normalizeBrowserTestOutput(''), '');
});

test('preview output remains bounded and cannot claim an unsupported status', () => {
  const input = {
    status: 'success',
    stdout: 'x'.repeat(20000),
    stderr: '',
    durationMs: 12.8,
    truncated: false
  };
  const result = boundedPreview(input);
  assert.equal(result.stdout.length, 16384);
  assert.equal(result.truncated, true);
  assert.equal(result.durationMs, 13);
  const unicode = boundedPreview({ ...input, stdout: '😀'.repeat(6000) });
  assert.equal(new TextEncoder().encode(unicode.stdout).length, 16384);
  assert.equal(unicode.stdout.includes('�'), false);
  assert.equal(unicode.truncated, true);
  assert.throws(() => boundedPreview({ ...input, status: 'graded' }));
  assert.equal(
    boundedPreview({ ...input, status: 'timeout' }).status,
    'timeout'
  );
  assert.throws(() => boundedPreview({ ...input, durationMs: NaN }));
  assert.throws(() => boundedPreview({ ...input, durationMs: -1 }));
  assert.throws(() => boundedPreview({ ...input, stdout: {} }));
  const image = `data:image/png;base64,${'A'.repeat(64)}`;
  assert.deepEqual(boundedPreview({ ...input, images: [image] }).images, [
    image
  ]);
  const invalidImage = boundedPreview({
    ...input,
    images: ['javascript:alert(1)'],
    truncated: false
  });
  assert.deepEqual(invalidImage.images, []);
  assert.equal(invalidImage.truncated, true);
  assert.equal(
    boundedPreview({
      ...input,
      images: Array.from({ length: 6 }, () => image),
      truncated: false
    }).truncated,
    true
  );
});

test('legacy runtime assets match their pinned checksums', () => {
  const root = new URL(
    '../../../apps/portal/static/runtimes/v1/clang/',
    import.meta.url
  );
  const manifest = JSON.parse(
    readFileSync(new URL('manifest.json', root), 'utf8')
  );
  for (const name of ['clang', 'lld', 'memfs', 'sysroot.tar', 'shared.js']) {
    const bytes = readFileSync(new URL(name, root));
    assert.equal(bytes.length, manifest.files[name].bytes);
    assert.equal(
      createHash('sha256').update(bytes).digest('hex'),
      manifest.files[name].sha256
    );
  }
});

test('public cache and cross-origin headers are restricted to versioned runtime assets', () => {
  const config = JSON.parse(
    readFileSync(
      new URL('../../../apps/portal/vercel.json', import.meta.url),
      'utf8'
    )
  );
  const blocks = new Map(
    config.headers.map((block: { source: string; headers: unknown[] }) => [
      block.source,
      Object.fromEntries(
        block.headers.map((header: { key: string; value: string }) => [
          header.key,
          header.value
        ])
      )
    ])
  );
  const v1 = blocks.get('/runtimes/v1/:path*') as Record<string, string>;
  assert.equal(v1['Cache-Control'], 'public, max-age=31536000, immutable');
  assert.equal(v1['Access-Control-Allow-Origin'], '*');
  assert.equal(v1['Access-Control-Allow-Credentials'], undefined);
  for (const name of ['clang', 'lld', 'memfs'])
    assert.equal(
      (blocks.get(`/runtimes/v1/clang/${name}`) as Record<string, string>)[
        'Content-Type'
      ],
      'application/wasm'
    );
  const v2 = blocks.get('/runtimes/v2/:path*') as Record<string, string>;
  assert.equal(v2['Cache-Control'], 'public, max-age=31536000, immutable');
  assert.equal(v2['Cross-Origin-Opener-Policy'], 'same-origin');
  assert.equal(v2['Cross-Origin-Embedder-Policy'], 'require-corp');
  assert.equal(v2['Cross-Origin-Resource-Policy'], 'cross-origin');
  assert.equal(v2['Access-Control-Allow-Credentials'], undefined);
  const v3 = blocks.get('/runtimes/v3/:path*') as Record<string, string>;
  assert.equal(v3['Cache-Control'], 'public, max-age=31536000, immutable');
  assert.equal(v3['Cross-Origin-Opener-Policy'], 'same-origin');
  assert.equal(v3['Cross-Origin-Embedder-Policy'], 'require-corp');
  assert.equal(v3['Cross-Origin-Resource-Policy'], 'cross-origin');
  assert.equal(v3['Access-Control-Allow-Credentials'], undefined);
});

test('runtime workers keep compile caching, watchdog and worker reuse contracts', () => {
  const root = new URL(
    '../../../apps/portal/static/runtimes/v3/',
    import.meta.url
  );
  const worker = readFileSync(new URL('worker.js', root), 'utf8');
  const frame = readFileSync(new URL('frame.js', root), 'utf8');
  assert.match(worker, /compileCache/);
  assert.match(worker, /compileStreaming/);
  assert.match(worker, /SharedArrayBuffer/);
  assert.match(worker, /loadPackagesFromImports/);
  assert.match(worker, /_async_input/);
  assert.match(worker, /__algohub_io/);
  assert.match(worker, /matplotlib/);
  assert.match(frame, /budgets = \{ c: 20000, python: 10000 \}/);
  assert.match(frame, /recycleAfter = \{ c: 200, python: 50 \}/);
  assert.match(frame, /Atomics\.store\(sabMeta, 0, 0\)/);
  assert.match(frame, /loadingLimit/);
  assert.match(frame, /imageLimit/);
  assert.match(frame, /pending\.language === 'python'/);
});
