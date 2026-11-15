import {
  ApiError,
  csrfToken,
  scopeContains,
  type Principal,
  type Scope
} from '@algohub/auth-client';
export type { Scope } from '@algohub/auth-client';
export type CourseKind =
  'pretest' | 'posttest' | 'keterampilan' | 'ujian_praktik';
export type QuestionType = 'essay' | 'coding';
export interface Session {
  id: string;
  title: string;
  sequence: number;
  scope: Scope;
  active: boolean;
  version: number;
}
export interface Course {
  id: string;
  session_id: string;
  title: string;
  kind: CourseKind;
  duration_minutes: number;
  token_required: boolean;
  version: number;
}
export interface Rubric {
  id: string;
  description: string;
  max_score: number;
}
export interface Question {
  id: string;
  course_id?: string;
  type: QuestionType;
  difficulty: 'easy' | 'medium' | 'hard';
  category: string;
  prompt: string;
  points: number;
  starter_code: string;
  language: string;
  reference_answer?: string;
  rubric?: Rubric[];
  version?: number;
}
export interface Activation {
  id: string;
  session_id: string;
  course_id: string;
  scope: Scope;
  timer_mode: 'shared' | 'individual';
  is_open: boolean;
  started_at: string | null;
  deadline_at: string | null;
  closes_at: string | null;
  question_count: number;
  version: number;
  student_ids?: number[];
  entry_approved_ids?: number[];
  question_ids?: string[];
  access_token?: string;
}
export interface SessionDetail {
  session: Session;
  courses: Course[];
  activations: Activation[];
}
export interface Answer {
  question_id: string;
  value: string;
  language?: string;
  updated_at?: string;
  score?: number | null;
  feedback?: string;
}
export interface Attempt {
  attempt_id: string;
  activation_id: string;
  session_id: string;
  course_id: string;
  user_id: number;
  status: 'in_progress' | 'submitted' | 'graded';
  submission_reason:
    'manual' | 'deadline' | 'closed' | 'legacy_submit' | '' | null;
  revision: number;
  grade_revision: number;
  grading_status: 'pending' | 'running' | 'needs_manual' | 'graded';
  started_at: string;
  deadline_at: string;
  submitted_at: string | null;
  server_now: string;
  timezone: string;
  timezone_offset: string;
  questions: Question[];
  answers: Answer[];
  score: number | null;
  max_score: number;
  percentage: number | null;
}
export interface GradeSuggestion {
  question_id: string;
  score: number;
  feedback: string;
}
export interface GradingJob {
  id: string;
  attempt_id: string;
  status: Attempt['grading_status'];
  grade_revision: number;
  suggestions: GradeSuggestion[];
  provider?: string;
  simulated: boolean;
  updated_at: string;
}
export interface AttemptReview {
  attempt: Attempt;
  questions: Question[];
  suggestions: GradeSuggestion[];
  grading_job: GradingJob | null;
}
export interface MonitoringRow {
  attempt_id: string;
  user_id: number;
  user_name: string;
  activation_id: string;
  session_id: string;
  course_id: string;
  status: Attempt['status'];
  grading_status: Attempt['grading_status'];
  answer_count: number;
  question_count: number;
  deadline_at: string;
  submitted_at: string | null;
}
export interface GradeSummary {
  status?: Attempt['status'];
  can_open?: boolean;
  attempt_id: string;
  session_id: string;
  session_title: string;
  course_title: string;
  course_kind: CourseKind;
  grading_status: Attempt['grading_status'];
  score: number | null;
  max_score: number;
  percentage: number | null;
  submitted_at: string | null;
}
export interface RunnerInfo {
  languages: { language: string; version: string; label: string }[];
  simulated: boolean;
  available: boolean;
}
export interface RunResult {
  status:
    | 'success'
    | 'compile_error'
    | 'runtime_error'
    | 'timeout'
    | 'quota_exceeded'
    | 'provider_error';
  stdout: string;
  stderr: string;
  exit_code: number | null;
  duration_ms: number;
  runner_version: string;
  request_id: string;
  source_hash: string;
  simulated: boolean;
  truncated: boolean;
}
export interface SaveBody {
  revision: number;
  answers: Answer[];
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
export function permitted(
  principal: Principal | null,
  permission: string,
  scope: Scope
): boolean {
  return !!principal?.grants.some(
    (grant) =>
      grant.permission === permission && scopeContains(grant.scope, scope)
  );
}
export const runLabels: Record<RunResult['status'], string> = {
  success: 'Berhasil',
  compile_error: 'Kompilasi gagal',
  runtime_error: 'Program berhenti karena error',
  timeout: 'Batas waktu terlampaui',
  quota_exceeded: 'Batas penggunaan tercapai',
  provider_error: 'Runner belum tersedia'
};
export const attemptLabels: Record<Attempt['status'], string> = {
  in_progress: 'Sedang dikerjakan',
  submitted: 'Jawaban terkumpul',
  graded: 'Nilai dipublikasikan'
};
export const gradingLabels: Record<Attempt['grading_status'], string> = {
  pending: 'Menunggu penilaian',
  running: 'Sedang ditinjau',
  needs_manual: 'Menunggu tinjauan pengajar',
  graded: 'Nilai dipublikasikan'
};
export const questionTypeLabels: Record<QuestionType, string> = {
  essay: 'Uraian',
  coding: 'Pemrograman'
};
export const difficultyLabels: Record<Question['difficulty'], string> = {
  easy: 'Dasar',
  medium: 'Menengah',
  hard: 'Lanjutan'
};
export function submissionLabel(reason: string | null): string {
  switch (reason) {
    case 'manual':
      return 'dikumpulkan oleh Anda';
    case 'deadline':
      return 'waktu pengerjaan habis';
    case 'closed':
      return 'aktivasi ditutup pengajar';
    case 'legacy_submit':
      return 'riwayat pengumpulan sebelumnya';
    default:
      return 'pengumpulan selesai';
  }
}
export const courseLabels: Record<CourseKind, string> = {
  pretest: 'Pretest',
  posttest: 'Posttest',
  keterampilan: 'Keterampilan',
  ujian_praktik: 'Ujian praktik'
};
export function jakarta(value: string | null): string {
  return value
    ? new Intl.DateTimeFormat('id-ID', {
        dateStyle: 'medium',
        timeStyle: 'medium',
        timeZone: 'Asia/Jakarta'
      }).format(new Date(value)) + ' WIB'
    : 'Belum ditetapkan';
}
export function errorMessage(error: unknown): string {
  if (!(error instanceof ApiError))
    return 'Koneksi terputus. Jawaban lokal tetap di memori; coba lagi tanpa menutup halaman.';
  const hint =
    error.status === 401
      ? 'Sesi login berakhir. Masuk kembali di tab lain, lalu coba lagi.'
      : error.status === 403
        ? 'Akses ditolak atau izin berubah. Periksa akun dan scope Anda.'
        : error.status === 409
          ? 'Data berubah di server. Muat data terbaru dan periksa sebelum mengirim ulang.'
          : error.status === 429
            ? 'Batas permintaan tercapai. Tunggu sebelum mencoba lagi.'
            : error.message;
  return `${hint}${error.requestId ? ` ID permintaan: ${error.requestId}` : ''}`;
}
export class PracticumClient {
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
        'Koneksi Praktikum terputus. Coba lagi tanpa menutup halaman.'
      );
    }
    const body = await response.json().catch(() => null);
    if (!response.ok || !body || typeof body !== 'object' || !('data' in body))
      throw new ApiError(
        body?.error?.message ?? 'Layanan Praktikum belum tersedia.',
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
  sessions(offset = 0): Promise<Session[]> {
    return this.get(`/practicum/sessions?limit=25&offset=${offset}`);
  }
  detail(id: string): Promise<SessionDetail> {
    return this.get(`/practicum/sessions/${encodeURIComponent(id)}`);
  }
  attempt(id: string): Promise<Attempt> {
    return this.get(`/practicum/attempts/${encodeURIComponent(id)}`);
  }
  grades(offset = 0): Promise<GradeSummary[]> {
    return this.get(`/practicum/grades?limit=25&offset=${offset}`);
  }
}
