function environmentPort(name: string, fallback: number): number {
  const value = process.env[name];
  if (!value) return fallback;
  if (!/^\d+$/.test(value)) throw new Error(`${name} must be a TCP port`);
  const port = Number(value);
  if (port < 1024 || port > 65535)
    throw new Error(`${name} must be between 1024 and 65535`);
  return port;
}

const appPortBase = environmentPort('ALGOHUB_E2E_APP_PORT_BASE', 5173);
if (appPortBase > 65530)
  throw new Error('ALGOHUB_E2E_APP_PORT_BASE must be at most 65530');

export const apiOrigin = `http://127.0.0.1:${environmentPort('ALGOHUB_E2E_API_PORT', 8080)}`;
export const hosts = {
  portal: `http://portal.localhost:${appPortBase}`,
  auth: `http://auth.localhost:${appPortBase + 1}`,
  admin: `http://admin.localhost:${appPortBase + 2}`,
  siakad: `http://siakad.localhost:${appPortBase + 3}`,
  elearning: `http://elearning.localhost:${appPortBase + 4}`,
  praktikum: `http://praktikum.localhost:${appPortBase + 5}`
} as const;

const systemKey =
  /^(path|pathext|systemroot|systemdrive|windir|comspec|temp|tmp|home|userprofile|appdata|localappdata|gocache|gomodcache|gopath|processor_architecture|number_of_processors|algohub_e2e_app_port_base|algohub_e2e_api_port)$/i;

export function systemEnvironment(): Record<string, string> {
  return Object.fromEntries(
    Object.entries(process.env).filter(
      ([key, value]) => systemKey.test(key) && value !== undefined
    )
  ) as Record<string, string>;
}

export function scrubbedEnvironment(): Record<string, string> {
  const safe = systemEnvironment();
  return Object.fromEntries(
    Object.keys(process.env).map((key) => [key, safe[key] ?? ''])
  );
}

export function fixtureEnvironment(): Record<string, string> {
  return {
    ...systemEnvironment(),
    ALGOHUB_ENV: 'local',
    ALGOHUB_ENABLE_FIXTURES: 'true',
    ALGOHUB_PRACTICUM_ENABLED: 'true',
    ALGOHUB_ELEARNING_ENABLED: 'true',
    ALGOHUB_SIAKAD_ENABLED: 'true',
    ALGOHUB_REQUEST_TIMEOUT: '25s',
    ALGOHUB_ADDR: new URL(apiOrigin).host,
    ALGOHUB_API_BASE_URL: apiOrigin,
    ALGOHUB_AUTH_ORIGIN: hosts.auth,
    ALGOHUB_SSO_ALLOWED_ORIGINS: Object.values(hosts).join(','),
    ALGOHUB_ALLOWED_ORIGINS: Object.values(hosts).join(','),
    ALGOHUB_DATABASE_URL: '',
    SUPABASE_URL: '',
    SUPABASE_ANON_KEY: '',
    ALGOHUB_AUTH_RATE_LIMIT: '10000',
    ALGOHUB_USER_RATE_LIMIT: '10000',
    ALGOHUB_TEST_MODE: 'true',
    GOENV: 'off',
    GOTELEMETRY: 'off'
  };
}
