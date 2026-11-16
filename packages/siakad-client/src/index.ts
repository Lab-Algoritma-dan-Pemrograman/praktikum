import {
  ApiError,
  csrfToken,
  scopeContains,
  type Principal,
  type Scope
} from '@algohub/auth-client';
export type { Scope } from '@algohub/auth-client';
export {
  emptyPage,
  loadResourcePage,
  mergeResources,
  PAGE_SIZE,
  type ResourcePage
} from './pagination.ts';

export type DayOfWeek =
  'Senin' | 'Selasa' | 'Rabu' | 'Kamis' | 'Jumat' | 'Sabtu';
export type AttendanceStatus = 'hadir' | 'izin' | 'alpha' | 'terlambat';
export type InventoryStatus = 'available' | 'maintenance' | 'retired';
export type RentalStatus = 'requested' | 'approved' | 'rejected' | 'returned';
export type RentalDecision = 'approve' | 'reject' | 'return' | 'verify_payment';

export interface AcademicClass {
  id: string;
  code: string;
  course_code: string;
  course_name: string;
  period_name: string;
  shift: number;
  student_ids?: number[];
  assistant_ids?: number[];
  active: boolean;
  scope: Scope;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface AcademicSchedule {
  id: string;
  class_id: string;
  class_code: string;
  course_name: string;
  day_of_week: DayOfWeek;
  shift: number;
  room: string;
  starts_at: string;
  ends_at: string;
  scope: Scope;
  version: number;
}

export interface AcademicAttendanceSession {
  id: string;
  schedule_id: string;
  class_id: string;
  title: string;
  is_open: boolean;
  opened_at: string;
  expires_at: string;
  closed_at: string | null;
  qr_token?: string;
  scope: Scope;
  version: number;
}

export interface AcademicScheduleView {
  schedule: AcademicSchedule;
  open_session: AcademicAttendanceSession | null;
}

export interface AcademicAttendanceRecord {
  id: string;
  session_id: string;
  schedule_id: string;
  class_id: string;
  user_id: number;
  user_name: string;
  status: AttendanceStatus;
  verified_by_qr: boolean;
  validated: boolean;
  checked_in_at: string;
  validated_by: number | null;
  validated_at: string | null;
  notes: string;
  scope: Scope;
  version: number;
}

export interface InventoryItem {
  id: string;
  code: string;
  name: string;
  category: string;
  total_quantity: number;
  available_quantity: number;
  rental_fee: number;
  status: InventoryStatus;
  scope: Scope;
  version: number;
  updated_at: string;
}

export interface InventoryRental {
  id: string;
  item_id: string;
  item_name: string;
  borrower_id: number;
  borrower_name: string;
  quantity: number;
  purpose: string;
  status: RentalStatus;
  total_fee: number;
  payment_verified: boolean;
  requested_at: string;
  due_at: string;
  decided_at: string | null;
  returned_at: string | null;
  decided_by: number | null;
  scope: Scope;
  version: number;
}

export interface AcademicNotification {
  id: string;
  user_id: number;
  type: string;
  title: string;
  message: string;
  read: boolean;
  created_at: string;
}

export interface AcademicLeaveRequest {
  id: string;
  user_id: number;
  user_name: string;
  schedule_id: string;
  class_id: string;
  kind: 'izin' | 'sakit' | 'reschedule';
  reason: string;
  original_at: string;
  status: 'pending' | 'approved' | 'rejected' | 'rescheduled';
  reschedule_at?: string | null;
  decided_at?: string | null;
  decided_by?: number | null;
  scope: Scope;
  version: number;
}

export interface AcademicFinanceMutation {
  id: string;
  user_id: number;
  user_name: string;
  type: 'income' | 'expense' | 'adjustment';
  note: string;
  amount: number;
  category: string;
  rental_id?: string;
  created_at: string;
  scope: Scope;
  version: number;
}

export interface AcademicFinanceBalance {
  user_id: number;
  user_name: string;
  saldo: number;
  income: number;
  expense: number;
  adjustment: number;
  mutations: AcademicFinanceMutation[];
  timezone: string;
}

export interface AcademicReportSummary {
  classes_count: number;
  schedules_count: number;
  open_attendance_sessions: number;
  attendance_by_status: Record<string, number>;
  total_inventory_items: number;
  available_inventory_units: number;
  active_rentals_count: number;
  total_verified_revenue: number;
  csv_export: string;
  generated_at: string;
  timezone: string;
  timezone_offset: string;
}

export interface Command {
  path: string;
  method: string;
  body: string;
  key: string;
}

export function command(path: string, method: string, body: unknown): Command {
  return { path, method, body: JSON.stringify(body), key: crypto.randomUUID() };
}

export function reusableCommand(
  previous: Command | null,
  path: string,
  method: string,
  body: unknown
): Command {
  return previous?.path === path &&
    previous.method === method &&
    previous.body === JSON.stringify(body)
    ? previous
    : command(path, method, body);
}

// A refreshed server version alone must not change an unresolved command's bytes/key.
export function versionedCommand(
  previous: Command | null,
  path: string,
  body: { version: number; [key: string]: unknown }
): Command {
  if (previous?.path === path && previous.method === 'PATCH') {
    const { version: _version, ...oldDraft } = JSON.parse(previous.body);
    const { version: _nextVersion, ...draft } = body;
    if (JSON.stringify(oldDraft) === JSON.stringify(draft)) return previous;
  }
  return reusableCommand(previous, path, 'PATCH', body);
}

// A 409 is a confirmed rejection; transport failures and 5xx can still hide a commit.
export function retainUnresolvedCommand<T extends Command>(
  previous: T | null,
  error: unknown
): T | null {
  return error instanceof ApiError && error.status === 409 ? null : previous;
}

export class PendingCommands {
  private entries = new Map<string, Command>();

  get(path: string, method: string): Command | null {
    return this.entries.get(`${method}:${path}`) ?? null;
  }

  put(mutation: Command): Command {
    this.entries.set(`${mutation.method}:${mutation.path}`, mutation);
    return mutation;
  }

  complete(mutation: Command): void {
    if (this.get(mutation.path, mutation.method) === mutation) {
      this.entries.delete(`${mutation.method}:${mutation.path}`);
    }
  }

  failed(mutation: Command, error: unknown): void {
    if (!retainUnresolvedCommand(mutation, error)) this.complete(mutation);
  }
}

export interface AttendanceDraftConflict {
  status: boolean;
  notes: boolean;
}

export function refreshAttendanceDrafts(
  previous: AcademicAttendanceRecord[],
  incoming: AcademicAttendanceRecord[],
  statuses: Record<string, AttendanceStatus>,
  notes: Record<string, string>,
  pending: PendingCommands,
  conflicts: Record<string, AttendanceDraftConflict> = {}
): {
  statuses: Record<string, AttendanceStatus>;
  notes: Record<string, string>;
  conflicts: Record<string, AttendanceDraftConflict>;
} {
  const oldRows = new Map(previous.map((row) => [row.id, row]));
  const nextStatuses = { ...statuses };
  const nextNotes = { ...notes };
  const nextConflicts = { ...conflicts };
  for (const row of incoming) {
    const old = oldRows.get(row.id);
    const localStatus = statuses[row.id] ?? old?.status ?? row.status;
    const localNotes = notes[row.id] ?? old?.notes ?? row.notes;
    if (
      pending.get(
        `/admin/siakad/absensi/records/${encodeURIComponent(row.id)}`,
        'PATCH'
      )
    ) {
      nextStatuses[row.id] = localStatus;
      nextNotes[row.id] = localNotes;
      continue;
    }
    const dirtyStatus = !!old && localStatus !== old.status;
    const dirtyNotes = !!old && localNotes !== old.notes;
    const priorConflict = conflicts[row.id];
    const statusConflict =
      localStatus !== row.status &&
      (!!priorConflict?.status ||
        (!!old && dirtyStatus && row.status !== old.status));
    const notesConflict =
      localNotes !== row.notes &&
      (!!priorConflict?.notes ||
        (!!old && dirtyNotes && row.notes !== old.notes));
    nextStatuses[row.id] =
      dirtyStatus || statusConflict ? localStatus : row.status;
    nextNotes[row.id] = dirtyNotes || notesConflict ? localNotes : row.notes;
    if (statusConflict || notesConflict) {
      nextConflicts[row.id] = { status: statusConflict, notes: notesConflict };
    } else {
      delete nextConflicts[row.id];
    }
  }
  return {
    statuses: nextStatuses,
    notes: nextNotes,
    conflicts: nextConflicts
  };
}

export interface RentalDraft {
  item_id: string;
  quantity: number;
  purpose: string;
  due_days: number;
}
export interface RentalCommand extends Command {
  draft: string;
}

export function rentalCommand(
  previous: RentalCommand | null,
  input: RentalDraft,
  now = Date.now()
): RentalCommand {
  const draft = JSON.stringify(input);
  if (previous?.draft === draft) return previous;
  const { due_days, ...body } = input;
  return {
    ...command('/inventory/rentals', 'POST', {
      ...body,
      due_at: new Date(now + due_days * 24 * 3600 * 1000).toISOString()
    }),
    draft
  };
}

export interface LeaveDraft {
  schedule_id: string;
  kind: 'izin' | 'sakit' | 'reschedule';
  reason: string;
  original_at: string;
}

// Leave and finance posts reuse the generic command helper so every mutation
// still carries an Idempotency-Key.
export function leaveCommand(
  previous: Command | null,
  input: LeaveDraft
): Command {
  return reusableCommand(previous, '/academic/izin', 'POST', input);
}

export function leaveDecisionCommand(
  previous: Command | null,
  id: string,
  body: { version: number; action: string; reschedule_at?: string }
): Command {
  return versionedCommand(
    previous,
    `/admin/siakad/izin/${encodeURIComponent(id)}`,
    body
  );
}

export function financeCommand(
  previous: Command | null,
  input: {
    target_user_id: number;
    type: 'income' | 'expense' | 'adjustment';
    note: string;
    amount: number;
    category?: string;
  }
): Command {
  return reusableCommand(previous, '/admin/siakad/keuangan', 'POST', input);
}

// Shared SIAKAD resources have empty division; a division-scoped grant (sekretaris, k3, labops, bendahara)
// authorizes its specific permission when the remaining scope dimensions are contained.
export function permitted(
  principal: Principal | null,
  permission: string,
  scope: Scope
): boolean {
  return !!principal?.grants.some((grant) => {
    if (grant.permission !== permission) return false;
    const target: Scope = {
      ...scope,
      ...(!scope.division && grant.scope.division
        ? { division: grant.scope.division }
        : {})
    };
    return scopeContains(grant.scope, target);
  });
}

export function jakarta(value: string | null): string {
  return value
    ? new Intl.DateTimeFormat('id-ID', {
        dateStyle: 'medium',
        timeStyle: 'medium',
        timeZone: 'Asia/Jakarta'
      }).format(new Date(value)) + ' WIB'
    : 'Belum ditetapkan';
}

export function rupiah(amount: number): string {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0
  }).format(amount);
}

export function errorMessage(error: unknown): string {
  if (!(error instanceof ApiError))
    return 'Koneksi SIAKAD terputus. Coba lagi tanpa menutup halaman.';
  const hint =
    error.status === 401
      ? 'Sesi login berakhir. Masuk kembali di tab lain, lalu coba lagi.'
      : error.status === 403
        ? 'Akses ditolak atau divisi/scope Anda tidak memiliki izin untuk aksi ini.'
        : error.status === 409
          ? error.name === 'ATTENDANCE_CLOSED' ||
            error.name === 'INSUFFICIENT_STOCK'
            ? error.message
            : 'Data berubah di server. Muat data terbaru dan periksa sebelum mengirim ulang.'
          : error.status === 429
            ? 'Batas permintaan tercapai. Tunggu sebelum mencoba lagi.'
            : error.message;
  return `${hint}${error.requestId ? ` ID permintaan: ${error.requestId}` : ''}`;
}

export class SiakadClient {
  private fetcher: typeof fetch;
  constructor(fetcher: typeof fetch = globalThis.fetch) {
    this.fetcher = fetcher.bind(globalThis);
  }
  private async request<T>(path: string, mutation?: Command): Promise<T> {
    let response: Response;
    try {
      response = await this.fetcher(`/api/v1${path}`, {
        method: mutation?.method ?? 'GET',
        credentials: 'same-origin',
        cache: 'no-store',
        redirect: 'error',
        signal: AbortSignal.timeout(35_000),
        headers: {
          Accept: 'application/json',
          ...(mutation
            ? {
                'Content-Type': 'application/json',
                'X-CSRF-Token': csrfToken(),
                'Idempotency-Key': mutation.key
              }
            : {})
        },
        ...(mutation ? { body: mutation.body } : {})
      });
    } catch {
      throw new ApiError(
        'Koneksi SIAKAD terputus. Coba lagi tanpa menutup halaman.'
      );
    }
    const body = await response.json().catch(() => null);
    if (!response.ok || !body || typeof body !== 'object' || !('data' in body))
      throw new ApiError(
        body?.error?.message ?? 'Layanan SIAKAD belum tersedia.',
        response.ok ? 502 : response.status,
        body?.error?.code ?? 'INVALID_RESPONSE',
        body?.meta?.request_id ?? response.headers.get('X-Request-ID') ?? ''
      );
    return body.data as T;
  }
  get<T>(path: string): Promise<T> {
    return this.request(path);
  }
  execute<T>(mutation: Command): Promise<T> {
    return this.request(mutation.path, mutation);
  }
  classes(offset = 0): Promise<AcademicClass[]> {
    return this.get(`/academic/classes?limit=25&offset=${offset}`);
  }
  schedules(offset = 0): Promise<AcademicScheduleView[]> {
    return this.get(`/academic/schedules?limit=25&offset=${offset}`);
  }
  attendance(offset = 0): Promise<AcademicAttendanceRecord[]> {
    return this.get(`/academic/attendance?limit=25&offset=${offset}`);
  }
  items(offset = 0): Promise<InventoryItem[]> {
    return this.get(`/inventory/items?limit=25&offset=${offset}`);
  }
  rentals(offset = 0): Promise<InventoryRental[]> {
    return this.get(`/inventory/rentals?limit=25&offset=${offset}`);
  }
  notifications(offset = 0): Promise<AcademicNotification[]> {
    return this.get(`/academic/notifications?limit=25&offset=${offset}`);
  }
}
