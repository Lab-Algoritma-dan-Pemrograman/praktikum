import {
  createHash,
  randomBytes,
  randomUUID,
  timingSafeEqual
} from 'node:crypto';
import {
  error,
  fail,
  isHttpError,
  json,
  redirect,
  type RequestEvent
} from '@sveltejs/kit';
import {
  safeRelativePath,
  safeReturnTo,
  type AppName,
  type ApplicationLink,
  type ApiEnvelope,
  type Principal
} from '@algohub/auth-client';

type IdentityEvent = Pick<
  RequestEvent,
  'url' | 'request' | 'cookies' | 'setHeaders'
>;
type Environment = Record<string, string | undefined>;
export const BODY_LIMIT = 65_536;
export const PRACTICUM_BODY_LIMIT = 262_144;
const RESPONSE_LIMIT = 1_048_576;
const PRACTICUM_RESPONSE_LIMIT = 4_194_304;
const TIMEOUT = 8_000;
const PRACTICUM_TIMEOUT = 30_000;
const idempotencyPattern = /^[A-Za-z0-9._:-]{1,128}$/;
const isPracticumPath = (path: string) =>
  path.startsWith('/v1/playground/') ||
  path.startsWith('/v1/practicum/') ||
  path.startsWith('/v1/admin/praktikum/') ||
  path.startsWith('/v1/learning/') ||
  path.startsWith('/v1/admin/elearning/') ||
  path.startsWith('/v1/academic/') ||
  path.startsWith('/v1/inventory/') ||
  path.startsWith('/v1/admin/siakad/');
const PENDING_TTL = 600;
const SESSION_COOKIE = 'algohub_session';
const CSRF_COOKIE = 'algohub_csrf';
const APP_PENDING = 'algohub_sso_pending';
const AUTH_PENDING = 'algohub_auth_pending';
const cookieNames = [SESSION_COOKIE, CSRF_COOKIE];
const productionOrigins = [
  'https://algohub.web.id',
  'https://portal.staging.algohub.web.id',
  ...['auth', 'admin', 'siakad', 'elearning', 'praktikum'].flatMap((name) => [
    `https://${name}.algohub.web.id`,
    `https://${name}.staging.algohub.web.id`
  ])
];
const opaque = /^[A-Za-z0-9_-]{1,2048}$/;
const challengePattern = /^[A-Za-z0-9_-]{43}$/;

export interface IdentityConfig {
  apiOrigin: string;
  appOrigin: string;
  authOrigin: string;
  allowedOrigins: string[];
  secure: boolean;
  fixtures: boolean;
  providerURL?: string;
  provider?: string;
}

export function isLocalHost(hostname: string): boolean {
  return (
    hostname === 'localhost' ||
    hostname.endsWith('.localhost') ||
    hostname === '127.0.0.1' ||
    hostname === '[::1]'
  );
}

function configuredOrigin(value: string, local: boolean): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw error(503, 'Origin identity belum dikonfigurasi dengan benar.');
  }
  if (
    url.username ||
    url.password ||
    url.pathname !== '/' ||
    url.search ||
    url.hash ||
    (url.protocol !== 'https:' &&
      !(local && url.protocol === 'http:' && isLocalHost(url.hostname)))
  ) {
    throw error(
      503,
      'Konfigurasi identity membutuhkan origin HTTPS yang valid.'
    );
  }
  return url.origin;
}

export function identityConfig(
  url: URL,
  env: Environment = process.env
): IdentityConfig {
  const local = env.ALGOHUB_ENV === 'local';
  const fixtures = env.ALGOHUB_ENABLE_FIXTURES === 'true';
  if (
    (local && !isLocalHost(url.hostname)) ||
    (fixtures && (!local || env.ALGOHUB_DATABASE_URL))
  )
    throw error(
      503,
      'Fixture identity hanya diizinkan pada environment lokal terisolasi.'
    );
  const allowedOrigins = (
    env.ALGOHUB_SSO_ALLOWED_ORIGINS ?? productionOrigins.join(',')
  )
    .split(',')
    .map((value) => configuredOrigin(value.trim(), local));
  const browserOrigins = (
    env.ALGOHUB_ALLOWED_ORIGINS ?? allowedOrigins.join(',')
  )
    .split(',')
    .map((value) => configuredOrigin(value.trim(), local));
  const appOrigin = configuredOrigin(url.origin, local);
  const authOrigin = configuredOrigin(
    env.ALGOHUB_AUTH_ORIGIN ?? 'https://auth.algohub.web.id',
    local
  );
  if (
    !browserOrigins.includes(appOrigin) ||
    !allowedOrigins.includes(appOrigin) ||
    !browserOrigins.includes(authOrigin) ||
    !allowedOrigins.includes(authOrigin)
  )
    throw error(
      503,
      'Origin aplikasi belum terdaftar pada konfigurasi identity.'
    );
  if (!env.ALGOHUB_API_BASE_URL)
    throw error(503, 'API identity belum dikonfigurasi.');
  const apiOrigin = configuredOrigin(env.ALGOHUB_API_BASE_URL, local);
  if (
    local &&
    [apiOrigin, ...allowedOrigins, ...browserOrigins].some(
      (origin) => !isLocalHost(new URL(origin).hostname)
    )
  )
    throw error(
      503,
      'Konfigurasi lokal tidak dapat menggunakan layanan nonlokal.'
    );
  const providerURL =
    !fixtures && env.SUPABASE_URL && env.SUPABASE_SSO_PROVIDER
      ? configuredOrigin(env.SUPABASE_URL, local)
      : undefined;
  if (local && providerURL && !isLocalHost(new URL(providerURL).hostname))
    throw error(503, 'Provider lokal harus menggunakan host lokal.');
  return {
    apiOrigin,
    appOrigin,
    authOrigin,
    allowedOrigins,
    secure: url.protocol === 'https:',
    fixtures,
    ...(providerURL ? { providerURL, provider: env.SUPABASE_SSO_PROVIDER } : {})
  };
}

export function applicationLinks(config: IdentityConfig): ApplicationLink[] {
  const labels: Record<AppName, string> = {
    portal: 'Portal',
    auth: 'Auth Center',
    admin: 'Admin',
    siakad: 'SIAKAD',
    elearning: 'E-learning',
    praktikum: 'Praktikum'
  };
  const local = isLocalHost(new URL(config.appOrigin).hostname);
  return (Object.keys(labels) as AppName[]).flatMap((name) => {
    const origin = config.allowedOrigins.find((origin) => {
      const host = new URL(origin).hostname;
      return name === 'portal'
        ? host === 'algohub.web.id' ||
            host === 'portal.staging.algohub.web.id' ||
            (local && host === 'portal.localhost')
        : name === 'auth'
          ? origin === config.authOrigin
          : host === `${name}.algohub.web.id` ||
            host === `${name}.staging.algohub.web.id` ||
            (local && host === `${name}.localhost`);
    });
    return origin ? [{ name, label: labels[name], origin }] : [];
  });
}

export function privateIdentityPage(
  event: IdentityEvent,
  referrerPolicy = 'no-referrer'
): void {
  event.setHeaders({
    'cache-control': 'no-store',
    'referrer-policy': referrerPolicy
  });
}

function requestID(event: IdentityEvent): string {
  const value = event.request.headers.get('x-request-id') ?? '';
  return /^[A-Za-z0-9._-]{1,64}$/.test(value) ? value : randomUUID();
}

const routes: Array<{
  path: RegExp;
  methods: string[];
  query?: string[];
  internal?: boolean;
}> = [
  { path: /^\/v1\/auth\/(me|sessions)$/, methods: ['GET'] },
  { path: /^\/v1\/auth\/sessions\/[A-Za-z0-9_-]{1,128}$/, methods: ['DELETE'] },
  { path: /^\/v1\/auth\/(refresh|logout)$/, methods: ['POST'] },
  { path: /^\/v1\/me$/, methods: ['GET', 'PATCH'] },
  { path: /^\/v1\/admin\/access$/, methods: ['GET'], query: ['path'] },
  {
    path: /^\/v1\/admin\/(overview|permissions|permission-catalog)$/,
    methods: ['GET']
  },
  {
    path: /^\/v1\/admin\/auditlog$/,
    methods: ['GET'],
    query: ['limit', 'before']
  },
  {
    path: /^\/v1\/admin\/users$/,
    methods: ['GET'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/users\/[1-9][0-9]*\/access$/,
    methods: ['GET', 'PUT']
  },
  {
    path: /^\/v1\/practicum\/(sessions|grades)$/,
    methods: ['GET'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/practicum\/sessions\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['GET']
  },
  { path: /^\/v1\/practicum\/attempts$/, methods: ['POST'] },
  {
    path: /^\/v1\/practicum\/attempts\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['GET']
  },
  {
    path: /^\/v1\/practicum\/attempts\/[A-Za-z0-9_-]{1,128}\/answers$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/practicum\/attempts\/[A-Za-z0-9_-]{1,128}\/(submit|run)$/,
    methods: ['POST']
  },
  { path: /^\/v1\/practicum\/runner$/, methods: ['GET'] },
  {
    path: /^\/v1\/admin\/praktikum\/sesi$/,
    methods: ['GET', 'POST'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/praktikum\/sesi\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['GET', 'PATCH']
  },
  { path: /^\/v1\/admin\/praktikum\/courses$/, methods: ['POST'] },
  {
    path: /^\/v1\/admin\/praktikum\/courses\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/praktikum\/questions$/,
    methods: ['GET', 'POST'],
    query: ['course_id', 'limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/praktikum\/questions\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/praktikum\/sesi\/[A-Za-z0-9_-]{1,128}\/activate$/,
    methods: ['POST']
  },
  {
    path: /^\/v1\/admin\/praktikum\/sesi\/[A-Za-z0-9_-]{1,128}\/courses\/[A-Za-z0-9_-]{1,128}\/close$/,
    methods: ['POST']
  },
  {
    path: /^\/v1\/admin\/praktikum\/monitoring$/,
    methods: ['GET'],
    query: ['session_id', 'activation_id', 'limit', 'offset']
  },
  { path: /^\/v1\/admin\/praktikum\/penilaian\/bulk$/, methods: ['POST'] },
  {
    path: /^\/v1\/admin\/praktikum\/penilaian\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['GET']
  },
  {
    path: /^\/v1\/admin\/praktikum\/penilaian\/[A-Za-z0-9_-]{1,128}\/grade$/,
    methods: ['POST']
  },
  {
    path: /^\/v1\/admin\/praktikum\/penilaian\/[A-Za-z0-9_-]{1,128}\/manual$/,
    methods: ['PUT']
  },
  {
    path: /^\/v1\/learning\/(levels|achievements|bug-hunt)$/,
    methods: ['GET']
  },
  {
    path: /^\/v1\/learning\/leaderboard$/,
    methods: ['GET'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/learning\/(modules|lessons)\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['GET']
  },
  {
    path: /^\/v1\/learning\/lessons\/[A-Za-z0-9_-]{1,128}\/progress$/,
    methods: ['PUT']
  },
  {
    path: /^\/v1\/learning\/(playground\/run|hints|bug-hunt\/attempts)$/,
    methods: ['POST']
  },
  { path: /^\/v1\/playground\/run$/, methods: ['POST'] },
  {
    path: /^\/v1\/admin\/elearning\/kurikulum$/,
    methods: ['GET']
  },
  {
    path: /^\/v1\/admin\/elearning\/(levels|modules|lessons)$/,
    methods: ['POST']
  },
  {
    path: /^\/v1\/admin\/elearning\/(levels|modules|lessons)\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/elearning\/bug-hunt$/,
    methods: ['GET', 'POST'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/elearning\/bug-hunt\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/elearning\/monitoring$/,
    methods: ['GET'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/academic\/(classes|schedules|notifications)$/,
    methods: ['GET'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/academic\/attendance$/,
    methods: ['GET', 'POST'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/inventory\/items$/,
    methods: ['GET'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/inventory\/rentals$/,
    methods: ['GET', 'POST'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/siakad\/(kelas|jadwal)$/,
    methods: ['GET', 'POST'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/siakad\/kelas\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/siakad\/absensi\/sesi$/,
    methods: ['POST']
  },
  {
    path: /^\/v1\/admin\/siakad\/absensi\/sesi\/[A-Za-z0-9_-]{1,128}\/close$/,
    methods: ['POST']
  },
  {
    path: /^\/v1\/admin\/siakad\/absensi\/records$/,
    methods: ['GET'],
    query: ['session_id', 'limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/siakad\/absensi\/records\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/siakad\/inventaris$/,
    methods: ['POST']
  },
  {
    path: /^\/v1\/admin\/siakad\/inventaris\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/siakad\/penyewaan$/,
    methods: ['GET'],
    query: ['limit', 'offset']
  },
  {
    path: /^\/v1\/admin\/siakad\/penyewaan\/[A-Za-z0-9_-]{1,128}$/,
    methods: ['PATCH']
  },
  {
    path: /^\/v1\/admin\/siakad\/laporan$/,
    methods: ['GET']
  },
  {
    path: /^\/v1\/auth\/(password|callback|sso\/prepare|sso\/exchange)$/,
    methods: ['POST'],
    internal: true
  }
];

function identityPath(
  path: string,
  method: string,
  browser = false,
  app?: AppName
): URL {
  if (!path.startsWith('/v1/') || /[\\#\s]/.test(path))
    throw error(404, 'Endpoint identity tidak tersedia.');
  const url = new URL(path, 'https://identity.invalid');
  const route = routes.find((route) => route.path.test(url.pathname));
  if (
    !route ||
    (browser && route.internal) ||
    (browser && app !== 'admin' && url.pathname.startsWith('/v1/admin/'))
  )
    throw error(404, 'Endpoint identity tidak tersedia.');
  if (!route.methods.includes(method))
    throw error(405, 'Metode tidak diizinkan.');
  for (const key of url.searchParams.keys()) {
    if (
      !route.query?.includes(key) ||
      url.searchParams.getAll(key).length !== 1
    )
      throw error(422, 'Parameter identity tidak valid.');
  }
  return url;
}

export function requireSameOrigin(
  event: IdentityEvent,
  config: IdentityConfig
): void {
  if (
    event.request.headers.get('origin') !== config.appOrigin ||
    event.request.headers.get('sec-fetch-site') === 'cross-site'
  )
    throw error(403, 'Permintaan harus berasal dari aplikasi yang sama.');
}

export async function readLimitedBody(
  source: Request | Response,
  limit = BODY_LIMIT,
  timeout = TIMEOUT
): Promise<string> {
  if (Number(source.headers.get('content-length') ?? '0') > limit)
    throw error(413, 'Ukuran permintaan melebihi batas.');
  const reader = source.body?.getReader();
  if (!reader) return '';
  let timer: ReturnType<typeof setTimeout> | undefined;
  const reading = (async () => {
    const chunks: Uint8Array[] = [];
    let size = 0;
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > limit) {
        void reader.cancel();
        throw error(413, 'Ukuran permintaan melebihi batas.');
      }
      chunks.push(value);
    }
    return Buffer.concat(chunks).toString('utf8');
  })();
  try {
    return await Promise.race([
      reading,
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => {
          void reader.cancel();
          try {
            error(504, 'Layanan identity melewati batas waktu.');
          } catch (cause) {
            reject(cause);
          }
        }, timeout);
      })
    ]);
  } finally {
    clearTimeout(timer);
  }
}

export function applySessionCookies(
  event: IdentityEvent,
  headers: Headers,
  config: IdentityConfig
): void {
  const values =
    headers.getSetCookie?.() ??
    headers.get('set-cookie')?.split(/,(?=\s*[^;,=\s]+=)/) ??
    [];
  for (const raw of values) {
    const [pair, ...attributes] = raw.split(';');
    const separator = pair.indexOf('=');
    const name = pair.slice(0, separator).trim();
    if (!cookieNames.includes(name)) continue;
    const value = pair.slice(separator + 1).trim();
    const options = new Map(
      attributes.map((part) => {
        const [key, ...value] = part.trim().split('=');
        return [key.toLowerCase(), value.join('=')];
      })
    );
    if (
      (value && !opaque.test(value)) ||
      options.has('domain') ||
      (options.has('path') && options.get('path') !== '/')
    )
      throw error(502, 'Cookie identity upstream tidak valid.');
    const maxAge = options.has('max-age')
      ? Number(options.get('max-age'))
      : undefined;
    const expires = options.has('expires')
      ? new Date(options.get('expires')!)
      : undefined;
    if (
      (maxAge !== undefined &&
        (!Number.isSafeInteger(maxAge) || maxAge > 31_536_000)) ||
      (expires && !Number.isFinite(expires.getTime()))
    )
      throw error(502, 'Masa berlaku cookie identity tidak valid.');
    event.cookies.set(name, value, {
      path: '/',
      httpOnly: name === SESSION_COOKIE,
      sameSite: 'lax',
      secure: config.secure,
      ...(maxAge !== undefined ? { maxAge: Math.max(0, maxAge) } : {}),
      ...(expires ? { expires } : {})
    });
  }
}

function withoutSecrets(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(withoutSecrets);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value)
      .filter(
        ([key]) =>
          !/^(access_token|refresh_token|id_token|provider_token|provider_refresh_token|password|code_verifier|service_role_key|auth_user_id|qr_token)$/i.test(
            key
          )
      )
      .map(([key, value]) => [key, withoutSecrets(value)])
  );
}

export async function identityRequest(
  event: IdentityEvent,
  path: string,
  init: {
    method?: string;
    body?: unknown;
    csrf?: string;
    idempotencyKey?: string;
  } = {}
): Promise<Response> {
  const config = identityConfig(event.url);
  const method = init.method ?? 'GET';
  const target = identityPath(path, method);
  const practicum = isPracticumPath(target.pathname);
  const bodyLimit = practicum ? PRACTICUM_BODY_LIMIT : BODY_LIMIT;
  const timeout = practicum ? PRACTICUM_TIMEOUT : TIMEOUT;
  const id = requestID(event);
  const headers = new Headers({
    accept: 'application/json',
    origin: config.appOrigin,
    'x-request-id': id
  });
  const cookies = cookieNames.flatMap((name) => {
    const value = event.cookies.get(name);
    return value && opaque.test(value) ? [`${name}=${value}`] : [];
  });
  if (cookies.length) headers.set('cookie', cookies.join('; '));
  const csrf = init.csrf ?? event.cookies.get(CSRF_COOKIE);
  if (csrf && opaque.test(csrf)) headers.set('x-csrf-token', csrf);
  if (practicum && method !== 'GET') {
    if (!init.idempotencyKey || !idempotencyPattern.test(init.idempotencyKey))
      throw error(
        422,
        'Idempotency-Key diperlukan (maksimal 128 karakter aman).'
      );
    headers.set('idempotency-key', init.idempotencyKey);
  }
  const body = init.body === undefined ? undefined : JSON.stringify(init.body);
  if (body !== undefined) {
    if (Buffer.byteLength(body) > bodyLimit)
      throw error(413, 'Ukuran permintaan melebihi batas.');
    headers.set('content-type', 'application/json');
  }
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);
  try {
    const upstream = await fetch(
      `${config.apiOrigin}${target.pathname}${target.search}`,
      { method, headers, body, redirect: 'error', signal: controller.signal }
    );
    const text = await readLimitedBody(
      upstream,
      practicum ? PRACTICUM_RESPONSE_LIMIT : RESPONSE_LIMIT,
      timeout
    );
    let envelope: {
      data?: unknown;
      error?: { code: string; message: string };
      meta?: ApiEnvelope<unknown>['meta'];
    };
    try {
      envelope = JSON.parse(text);
    } catch {
      throw error(502, 'Respons layanan identity tidak valid.');
    }
    if (
      !envelope ||
      typeof envelope !== 'object' ||
      (upstream.ok ? !('data' in envelope) : !envelope.error)
    )
      throw error(502, 'Respons layanan identity tidak valid.');
    applySessionCookies(event, upstream.headers, config);
    const upstreamID =
      upstream.headers.get('x-request-id') ?? envelope.meta?.request_id ?? id;
    const responseID = /^[A-Za-z0-9._-]{1,64}$/.test(upstreamID)
      ? upstreamID
      : id;
    const upstreamOperation =
      upstream.headers.get('x-operation-id') ??
      envelope.meta?.operation_id ??
      'identity.bff';
    const operation = /^[A-Za-z0-9._-]{1,100}$/.test(upstreamOperation)
      ? upstreamOperation
      : 'identity.bff';
    const responseHeaders: Record<string, string> = {
      'cache-control': 'no-store',
      'referrer-policy': 'no-referrer',
      'x-request-id': responseID,
      'x-operation-id': operation
    };
    const retryAfter = upstream.headers.get('retry-after');
    if (
      upstream.status === 429 &&
      retryAfter &&
      /^[1-9][0-9]{0,4}$/.test(retryAfter)
    ) {
      responseHeaders['retry-after'] = retryAfter;
    }
    // Only an activation-creation response may expose its one-time classroom
    // token. OAuth/session credentials remain filtered on every route.
    const data = withoutSecrets(envelope.data);
    if (
      method === 'POST' &&
      upstream.status === 201 &&
      /^\/v1\/admin\/praktikum\/sesi\/[A-Za-z0-9_-]{1,128}\/activate$/.test(
        target.pathname
      ) &&
      data &&
      typeof data === 'object' &&
      !Array.isArray(data)
    ) {
      const accessToken = (envelope.data as Record<string, unknown>)
        ?.access_token;
      if (
        typeof accessToken === 'string' &&
        /^[A-Za-z0-9_-]{1,128}$/.test(accessToken)
      )
        (data as Record<string, unknown>).access_token = accessToken;
    }
    if (
      method === 'POST' &&
      upstream.status === 201 &&
      target.pathname === '/v1/admin/siakad/absensi/sesi' &&
      data &&
      typeof data === 'object' &&
      !Array.isArray(data)
    ) {
      const qrToken = (envelope.data as Record<string, unknown>)?.qr_token;
      if (typeof qrToken === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(qrToken))
        (data as Record<string, unknown>).qr_token = qrToken;
    }
    return json(
      {
        ...(upstream.ok
          ? { data }
          : {
              error: {
                code:
                  typeof envelope.error?.code === 'string'
                    ? envelope.error.code
                    : 'IDENTITY_ERROR',
                message:
                  typeof envelope.error?.message === 'string'
                    ? envelope.error.message
                    : 'Permintaan identity gagal.'
              }
            }),
        meta: {
          request_id: responseID,
          operation_id: operation,
          ...(Number.isSafeInteger(envelope.meta?.next_before) &&
          envelope.meta!.next_before! >= 0
            ? { next_before: envelope.meta!.next_before }
            : {})
        }
      },
      {
        status: upstream.status,
        headers: responseHeaders
      }
    );
  } catch (cause) {
    if (isHttpError(cause)) throw cause;
    throw error(
      controller.signal.aborted ? 504 : 502,
      'Layanan identity belum dapat dijangkau. Coba lagi.'
    );
  } finally {
    clearTimeout(timer);
  }
}

export async function identityEnvelope<T>(
  event: IdentityEvent,
  path: string,
  init: { method?: string; body?: unknown } = {}
): Promise<ApiEnvelope<T>> {
  const response = await identityRequest(event, path, init);
  const envelope = await response.json();
  if (!response.ok)
    throw error(
      response.status,
      `${envelope.error?.message ?? 'Permintaan identity gagal.'} (Request ID: ${envelope.meta.request_id})`
    );
  return envelope as ApiEnvelope<T>;
}

export async function identityData<T>(
  event: IdentityEvent,
  path: string,
  init: { method?: string; body?: unknown } = {}
): Promise<T> {
  return (await identityEnvelope<T>(event, path, init)).data;
}

export async function proxyIdentity(
  event: IdentityEvent,
  app: AppName
): Promise<Response> {
  try {
    const config = identityConfig(event.url);
    const path = `${event.url.pathname.slice('/api'.length)}${event.url.search}`;
    identityPath(path, event.request.method, true, app);
    const origin = event.request.headers.get('origin');
    if (origin && origin !== config.appOrigin)
      throw error(403, 'Origin tidak diizinkan.');
    let body: unknown;
    let csrf: string | undefined;
    if (event.request.method !== 'GET') {
      requireSameOrigin(event, config);
      csrf = event.request.headers.get('x-csrf-token') ?? '';
      if (!opaque.test(csrf))
        throw error(403, 'CSRF token diperlukan. Muat ulang sesi Anda.');
      if (
        event.request.headers.get('content-type')?.split(';')[0] !==
        'application/json'
      )
        throw error(415, 'Gunakan application/json.');
      const text = await readLimitedBody(
        event.request,
        isPracticumPath(event.url.pathname.slice('/api'.length))
          ? PRACTICUM_BODY_LIMIT
          : BODY_LIMIT
      );
      if (text) {
        try {
          body = JSON.parse(text);
        } catch {
          throw error(422, 'Body JSON tidak valid.');
        }
      }
    }
    return await identityRequest(event, path, {
      method: event.request.method,
      body,
      csrf,
      idempotencyKey: event.request.headers.get('idempotency-key') ?? undefined
    });
  } catch (cause) {
    const status = isHttpError(cause) ? cause.status : 503;
    const id = requestID(event);
    return json(
      {
        error: {
          code: `IDENTITY_${status}`,
          message: isHttpError(cause)
            ? cause.body.message
            : 'Identity belum tersedia.'
        },
        meta: { request_id: id, operation_id: 'identity.bff' }
      },
      { status, headers: { 'cache-control': 'no-store', 'x-request-id': id } }
    );
  }
}

interface PendingLogin {
  returnTo: string | null;
  challenge?: string;
  verifier?: string;
  nonce: string;
  expiresAt: number;
}

function writePending(
  event: IdentityEvent,
  name: string,
  pending: PendingLogin,
  path: string
): void {
  event.cookies.set(
    name,
    Buffer.from(JSON.stringify(pending)).toString('base64url'),
    {
      path,
      httpOnly: true,
      sameSite: 'lax',
      secure: identityConfig(event.url).secure,
      maxAge: PENDING_TTL
    }
  );
}

function pendingLogin(event: IdentityEvent, name: string): PendingLogin {
  const raw = event.cookies.get(name);
  try {
    if (!raw || raw.length > 4096) throw new Error();
    const pending = JSON.parse(
      Buffer.from(raw, 'base64url').toString()
    ) as PendingLogin;
    if (
      !pending ||
      !challengePattern.test(pending.nonce) ||
      !Number.isFinite(pending.expiresAt) ||
      pending.expiresAt <= Date.now() ||
      pending.expiresAt > Date.now() + PENDING_TTL * 1000
    )
      throw new Error();
    if (
      pending.returnTo !== null &&
      safeReturnTo(
        pending.returnTo,
        identityConfig(event.url).allowedOrigins
      ) !== pending.returnTo
    )
      throw new Error();
    return pending;
  } catch {
    throw error(
      400,
      'Permintaan login tidak tersedia atau kedaluwarsa. Mulai login kembali.'
    );
  }
}

export function pkce(): { verifier: string; challenge: string } {
  const verifier = randomBytes(32).toString('base64url');
  return {
    verifier,
    challenge: createHash('sha256').update(verifier).digest('base64url')
  };
}

export function startAppSSO(event: IdentityEvent): never {
  privateIdentityPage(event);
  const config = identityConfig(event.url);
  const path = safeRelativePath(event.url.searchParams.get('return_to') ?? '/');
  if (!path || event.url.searchParams.getAll('return_to').length > 1)
    throw error(422, 'Tujuan login tidak diizinkan.');
  const { verifier, challenge } = pkce();
  const returnTo = `${config.appOrigin}${path}`;
  writePending(
    event,
    APP_PENDING,
    {
      returnTo,
      verifier,
      nonce: randomBytes(32).toString('base64url'),
      expiresAt: Date.now() + PENDING_TTL * 1000
    },
    '/callback'
  );
  const login = new URL('/login', config.authOrigin);
  login.searchParams.set('return_to', returnTo);
  login.searchParams.set('code_challenge', challenge);
  throw redirect(303, login.toString(), { external: [config.authOrigin] });
}

export async function completeAppSSO(event: IdentityEvent): Promise<never> {
  privateIdentityPage(event);
  const config = identityConfig(event.url);
  const pending = pendingLogin(event, APP_PENDING);
  event.cookies.delete(APP_PENDING, {
    path: '/callback',
    secure: config.secure
  });
  const code = event.url.searchParams.get('code') ?? '';
  const returnTo = event.url.searchParams.get('return_to') ?? '';
  if (
    !opaque.test(code) ||
    event.url.searchParams.size !== 2 ||
    !pending.verifier ||
    !challengePattern.test(pending.verifier) ||
    returnTo !== pending.returnTo ||
    new URL(returnTo).origin !== config.appOrigin
  )
    throw error(
      400,
      'Callback tidak cocok dengan permintaan login aplikasi ini.'
    );
  await identityData(event, '/v1/auth/sso/exchange', {
    method: 'POST',
    body: { code, return_to: returnTo, code_verifier: pending.verifier }
  });
  throw redirect(303, returnTo, { external: [config.appOrigin] });
}

function centralConfig(event: IdentityEvent): IdentityConfig {
  const config = identityConfig(event.url);
  if (config.appOrigin !== config.authOrigin)
    throw error(403, 'Login hanya tersedia pada Auth Center.');
  return config;
}

async function finishCentralLogin(
  event: IdentityEvent,
  pending: PendingLogin
): Promise<never> {
  const config = centralConfig(event);
  if (!pending.returnTo) {
    event.cookies.delete(AUTH_PENDING, { path: '/', secure: config.secure });
    throw redirect(303, '/');
  }
  if (!pending.challenge || !challengePattern.test(pending.challenge))
    throw error(400, 'PKCE aplikasi tidak valid.');
  const handshake = await identityData<{ code: string }>(
    event,
    '/v1/auth/sso/prepare',
    {
      method: 'POST',
      body: { return_to: pending.returnTo, code_challenge: pending.challenge }
    }
  );
  if (!opaque.test(handshake.code))
    throw error(502, 'Kode SSO tidak dapat diterbitkan.');
  event.cookies.delete(AUTH_PENDING, { path: '/', secure: config.secure });
  const callback = new URL('/callback', new URL(pending.returnTo).origin);
  callback.searchParams.set('code', handshake.code);
  callback.searchParams.set('return_to', pending.returnTo);
  throw redirect(303, callback.toString(), { external: config.allowedOrigins });
}

export async function centralLoginLoad(event: IdentityEvent) {
  privateIdentityPage(event, 'same-origin');
  const config = centralConfig(event);
  const submitted =
    event.request.method === 'POST' ? pendingLogin(event, AUTH_PENDING) : null;
  const raw = submitted
    ? submitted.returnTo
    : event.url.searchParams.get('return_to');
  const challenge = submitted
    ? submitted.challenge
    : (event.url.searchParams.get('code_challenge') ?? undefined);
  const returnTo = raw ? safeReturnTo(raw, config.allowedOrigins) : null;
  if (
    (raw !== null &&
      (!returnTo || !challenge || !challengePattern.test(challenge))) ||
    (!raw && challenge) ||
    [...event.url.searchParams.keys()].some(
      (key) =>
        !['return_to', 'code_challenge', '/password', '/oauth'].includes(key) ||
        event.url.searchParams.getAll(key).length !== 1
    )
  )
    throw error(422, 'Tujuan atau PKCE login tidak diizinkan.');
  const pending: PendingLogin = {
    returnTo,
    challenge,
    nonce: randomBytes(32).toString('base64url'),
    expiresAt: Date.now() + PENDING_TTL * 1000
  };
  const session = await identityRequest(event, '/v1/auth/me');
  if (session.ok) return finishCentralLogin(event, pending);
  if (session.status !== 401)
    throw error(
      session.status,
      'Sesi identity belum dapat diperiksa. Coba lagi.'
    );
  writePending(event, AUTH_PENDING, pending, '/');
  return {
    loginToken: pending.nonce,
    returnTo,
    fixtures: config.fixtures,
    oauthAvailable: !!config.providerURL
  };
}

async function loginForm(
  event: IdentityEvent
): Promise<{ pending: PendingLogin; form: URLSearchParams }> {
  const config = centralConfig(event);
  requireSameOrigin(event, config);
  if (
    event.request.headers.get('content-type')?.split(';')[0] !==
    'application/x-www-form-urlencoded'
  )
    throw error(415, 'Format login tidak didukung.');
  const pending = pendingLogin(event, AUTH_PENDING);
  const form = new URLSearchParams(await readLimitedBody(event.request, 8192));
  const nonce = form.get('login_token') ?? '';
  if (
    !challengePattern.test(nonce) ||
    !timingSafeEqual(Buffer.from(nonce), Buffer.from(pending.nonce)) ||
    form.getAll('login_token').length !== 1
  )
    throw error(403, 'Permintaan login tidak cocok. Muat ulang halaman.');
  return { pending, form };
}

const nimPattern = /^[0-9]{6,24}$/;

function loginEmail(identifier: string, env: Environment): string {
  const value = identifier.trim().toLowerCase();
  if (value.includes('@')) return value;
  const domain = env.ALGOHUB_LOGIN_EMAIL_DOMAIN ?? 'users.algohub.web.id';
  return nimPattern.test(value) ? `${value}@${domain}` : '';
}

export async function passwordLogin(event: IdentityEvent) {
  const { pending, form } = await loginForm(event);
  const email = loginEmail(form.get('email') ?? '', process.env);
  const password = form.get('password') ?? '';
  if (!email || email.length > 254 || !password || password.length > 1024)
    return fail(422, {
      message: 'Isi email atau NIM, dan password yang valid.'
    });
  const response = await identityRequest(event, '/v1/auth/password', {
    method: 'POST',
    body: { email, password }
  });
  if (!response.ok) {
    const envelope = await response.json();
    return fail(response.status, {
      message:
        response.status === 401
          ? 'Email atau password tidak sesuai.'
          : 'Login belum berhasil. Coba lagi.',
      requestId: envelope.meta.request_id
    });
  }
  return finishCentralLogin(event, pending);
}

export async function startProviderLogin(event: IdentityEvent): Promise<never> {
  const { pending } = await loginForm(event);
  const config = centralConfig(event);
  if (!config.providerURL || !config.provider)
    throw error(503, 'Provider OAuth belum dikonfigurasi.');
  const { verifier, challenge } = pkce();
  writePending(event, AUTH_PENDING, { ...pending, verifier }, '/');
  const authorize = new URL('/auth/v1/authorize', config.providerURL);
  authorize.searchParams.set('provider', config.provider);
  authorize.searchParams.set('code_challenge', challenge);
  authorize.searchParams.set('code_challenge_method', 's256');
  authorize.searchParams.set('redirect_to', `${config.authOrigin}/callback`);
  throw redirect(303, authorize.toString(), { external: [config.providerURL] });
}

export async function completeCentralCallback(
  event: IdentityEvent
): Promise<never> {
  if (event.url.searchParams.has('return_to')) return completeAppSSO(event);
  privateIdentityPage(event);
  const config = centralConfig(event);
  const pending = pendingLogin(event, AUTH_PENDING);
  event.cookies.delete(AUTH_PENDING, { path: '/', secure: config.secure });
  const code = event.url.searchParams.get('code') ?? '';
  if (
    !opaque.test(code) ||
    event.url.searchParams.size !== 1 ||
    !pending.verifier ||
    !challengePattern.test(pending.verifier)
  )
    throw error(400, 'Callback provider tidak valid. Mulai login kembali.');
  await identityData(event, '/v1/auth/callback', {
    method: 'POST',
    body: { code, code_verifier: pending.verifier }
  });
  return finishCentralLogin(event, pending);
}

export async function loadAppIdentity(event: IdentityEvent) {
  privateIdentityPage(event);
  const config = identityConfig(event.url);
  const response = await identityRequest(event, '/v1/auth/me');
  if (!response.ok && response.status !== 401)
    throw error(
      response.status,
      'Sesi identity belum dapat dimuat. Coba lagi.'
    );
  const principal = response.ok
    ? ((await response.json()).data as Principal)
    : null;
  if (principal) {
    principal.assignments ??= [];
    principal.grants ??= [];
  }
  return {
    principal,
    applications: applicationLinks(config),
    authOrigin: config.authOrigin
  };
}
