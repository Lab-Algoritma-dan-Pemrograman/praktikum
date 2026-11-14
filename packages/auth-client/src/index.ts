export type RoleCode = 'koordinator' | 'asisten' | 'mahasiswa' | 'peminjam';
export type DivisionCode =
  'sekretaris' | 'bendahara' | 'k3' | 'labops' | 'pendidikan' | 'pdd';
export type AppName =
  'portal' | 'auth' | 'admin' | 'siakad' | 'elearning' | 'praktikum';

export interface Scope {
  domain?: string;
  institution?: string;
  division?: string;
  class_id?: string;
  shift?: number;
  session_id?: string;
}

export interface UserProfile {
  id: number;
  email: string;
  name: string;
  nim?: string;
  is_active: boolean;
  access_version: number;
}

export interface RoleAssignment {
  id: number;
  user_id: number;
  role: RoleCode;
  division?: DivisionCode;
  scope: Scope;
  active: boolean;
}

export interface PermissionGrant {
  permission: string;
  app: string;
  role: RoleCode;
  division?: DivisionCode;
  scope: Scope;
}

export interface Principal {
  user: UserProfile;
  assignments: RoleAssignment[];
  grants: PermissionGrant[];
}

export interface SessionView {
  id: string;
  host: string;
  created_at: string;
  expires_at: string;
  revoked: boolean;
}

export interface AuditEvent {
  id: number;
  actor_id?: number;
  action: string;
  app: string;
  subject_type: string;
  subject_id: string;
  scope: Scope;
  request_id: string;
  operation_id: string;
  detail?: Record<string, unknown>;
  created_at: string;
}

export interface PermissionDefinition {
  code: string;
  app: string;
  description: string;
}

export interface UserAccess {
  version: number;
  assignments: RoleAssignment[];
  grants: PermissionGrant[];
}

export interface AccessUpdate {
  version: number;
  assignments: Array<Pick<RoleAssignment, 'role' | 'division' | 'scope'>>;
  grants: Array<
    Pick<PermissionGrant, 'permission' | 'role' | 'division' | 'scope'>
  >;
}

export interface ApplicationLink {
  name: AppName;
  label: string;
  origin: string;
}

export interface AppIdentityData {
  principal: Principal | null;
  applications: ApplicationLink[];
  authOrigin: string;
}

export interface ApiEnvelope<T> {
  data: T;
  meta: { request_id: string; operation_id: string; next_before?: number };
}

export class ApiError extends Error {
  status: number;
  requestId: string;
  operationId: string;

  constructor(
    message: string,
    status = 503,
    code = 'UNAVAILABLE',
    requestId = '',
    operationId = ''
  ) {
    super(message);
    this.name = code;
    this.status = status;
    this.requestId = requestId;
    this.operationId = operationId;
  }
}

export function asApiError(error: unknown): ApiError {
  return error instanceof ApiError
    ? error
    : new ApiError('Layanan belum dapat dijangkau. Coba lagi.');
}

export function csrfToken(
  cookie = typeof document === 'undefined' ? '' : document.cookie
): string {
  const value = cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith('algohub_csrf='));
  try {
    return value ? decodeURIComponent(value.slice('algohub_csrf='.length)) : '';
  } catch {
    return '';
  }
}

export class AuthClient {
  private readonly fetcher: typeof fetch;

  constructor(fetcher: typeof fetch = globalThis.fetch) {
    this.fetcher = fetcher.bind(globalThis);
  }

  private async requestEnvelope<T>(
    path: string,
    method = 'GET',
    data?: unknown
  ): Promise<ApiEnvelope<T>> {
    let response: Response;
    try {
      response = await this.fetcher(`/api${path}`, {
        method,
        credentials: 'same-origin',
        cache: 'no-store',
        redirect: 'error',
        signal: AbortSignal.timeout(12_000),
        headers: {
          Accept: 'application/json',
          ...(method !== 'GET'
            ? {
                'X-CSRF-Token': csrfToken(),
                'Content-Type': 'application/json'
              }
            : {})
        },
        ...(data !== undefined ? { body: JSON.stringify(data) } : {})
      });
    } catch {
      throw new ApiError('Koneksi ke layanan identity terputus. Coba lagi.');
    }
    const body = (await response.json().catch(() => null)) as
      (ApiEnvelope<T> & { error?: { code: string; message: string } }) | null;
    if (
      !response.ok ||
      !body ||
      typeof body !== 'object' ||
      !('data' in body)
    ) {
      throw new ApiError(
        body?.error?.message ?? 'Permintaan belum dapat diproses.',
        response.ok ? 502 : response.status,
        body?.error?.code ?? 'INVALID_RESPONSE',
        body?.meta?.request_id ?? response.headers.get('X-Request-ID') ?? '',
        body?.meta?.operation_id ?? ''
      );
    }
    return body;
  }

  private async request<T>(
    path: string,
    method = 'GET',
    data?: unknown
  ): Promise<T> {
    return (await this.requestEnvelope<T>(path, method, data)).data;
  }

  me(): Promise<Principal> {
    return this.request('/v1/auth/me');
  }
  profile(): Promise<UserProfile> {
    return this.request('/v1/me');
  }
  updateProfile(name: string): Promise<UserProfile> {
    return this.request('/v1/me', 'PATCH', { name });
  }
  sessions(): Promise<SessionView[]> {
    return this.request('/v1/auth/sessions');
  }
  revokeSession(id: string): Promise<{ revoked: boolean }> {
    return this.request(
      `/v1/auth/sessions/${encodeURIComponent(id)}`,
      'DELETE'
    );
  }
  refresh(): Promise<{ user: UserProfile }> {
    return this.request('/v1/auth/refresh', 'POST');
  }
  logout(): Promise<{ logged_out: boolean }> {
    return this.request('/v1/auth/logout', 'POST');
  }
  access(path: string): Promise<Principal> {
    return this.request(`/v1/admin/access?${new URLSearchParams({ path })}`);
  }
  permissions(): Promise<Pick<Principal, 'assignments' | 'grants'>> {
    return this.request('/v1/admin/permissions');
  }
  async audit(
    before?: number
  ): Promise<{ events: AuditEvent[]; nextBefore: number }> {
    const result = await this.requestEnvelope<AuditEvent[]>(
      `/v1/admin/auditlog?limit=50${before ? `&before=${before}` : ''}`
    );
    return { events: result.data, nextBefore: result.meta.next_before ?? 0 };
  }
  users(offset = 0): Promise<UserProfile[]> {
    return this.request(`/v1/admin/users?limit=25&offset=${offset}`);
  }
  userAccess(id: number): Promise<UserAccess> {
    return this.request(`/v1/admin/users/${id}/access`);
  }
  updateAccess(id: number, access: AccessUpdate): Promise<UserAccess> {
    return this.request(`/v1/admin/users/${id}/access`, 'PUT', access);
  }
}

export const adminNavigation = [
  { href: '/', label: 'Ringkasan', permissions: ['admin.overview.read'] },
  {
    href: '/praktikum',
    label: 'Praktikum',
    permissions: [
      'practicum.session.manage',
      'question.manage',
      'practicum.grade.write',
      'schedule.approve'
    ]
  },
  {
    href: '/elearning',
    label: 'E-learning',
    permissions: ['learning.content.manage', 'learning.monitor']
  },
  {
    href: '/siakad',
    label: 'SIAKAD',
    permissions: [
      'class.manage',
      'schedule.write',
      'attendance.read',
      'inventory.manage',
      'rental.approve',
      'finance.manage'
    ]
  },
  { href: '/auditlog', label: 'Audit log', permissions: ['audit.read'] },
  { href: '/profil', label: 'Profil', permissions: ['admin.overview.read'] },
  {
    href: '/pengaturan',
    label: 'Pengaturan',
    permissions: ['identity.access.manage']
  }
];

export function hasPermission(
  grants: PermissionGrant[],
  permission: string,
  domain?: string
): boolean {
  return grants.some(
    (grant) =>
      grant.permission === permission &&
      (!domain ||
        !grant.scope.domain ||
        grant.scope.domain === '*' ||
        grant.scope.domain === domain)
  );
}

export function scopeContains(granted: Scope, selected: Scope): boolean {
  return Object.entries(granted).every(
    ([key, value]) =>
      !value || value === '*' || selected[key as keyof Scope] === value
  );
}

export function scopeLabel(scope: Scope): string {
  const labels: Record<keyof Scope, string> = {
    domain: 'Domain',
    institution: 'Institusi',
    division: 'Divisi',
    class_id: 'Kelas',
    shift: 'Shift',
    session_id: 'Sesi'
  };
  const fields = Object.entries(scope).filter(
    ([, value]) => value && value !== '*'
  );
  return fields.length
    ? fields
        .map(([key, value]) => `${labels[key as keyof Scope]}: ${value}`)
        .join(' · ')
    : 'Seluruh scope yang diberikan';
}

export function canAccessAdminRoute(
  principal: Principal,
  path: string
): boolean {
  if (
    !principal.assignments.some(
      (assignment) =>
        assignment.active &&
        ['koordinator', 'asisten'].includes(assignment.role)
    ) ||
    !hasPermission(principal.grants, 'admin.overview.read')
  )
    return false;
  const item = adminNavigation.find((item) => item.href === path);
  if (!item) return false;
  if (path === '/pengaturan')
    return (
      principal.assignments.some(
        (assignment) =>
          assignment.active &&
          assignment.role === 'koordinator' &&
          scopeContains(assignment.scope, {})
      ) &&
      principal.grants.some(
        (grant) =>
          grant.permission === 'identity.access.manage' &&
          scopeContains(grant.scope, {})
      )
    );
  const domain = ['/praktikum', '/elearning', '/siakad'].includes(path)
    ? path.slice(1)
    : undefined;
  return item.permissions.some((permission) =>
    hasPermission(principal.grants, permission, domain)
  );
}

export function safeRelativePath(value: string): string | null {
  return adminNavigation.some((item) => item.href === value) ? value : null;
}

export function safeReturnTo(
  value: string,
  allowedOrigins: string[]
): string | null {
  if (/[\\\s%?#]/.test(value)) return null;
  try {
    const url = new URL(value);
    if (
      !['https:', 'http:'].includes(url.protocol) ||
      url.username ||
      url.password ||
      !allowedOrigins.includes(url.origin) ||
      !safeRelativePath(url.pathname)
    )
      return null;
    if (value !== `${url.origin}${url.pathname}` && value !== url.origin)
      return null;
    return `${url.origin}${url.pathname}`;
  } catch {
    return null;
  }
}
