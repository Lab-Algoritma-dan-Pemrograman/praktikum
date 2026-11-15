import {
  ApiError,
  csrfToken,
  scopeContains,
  type Principal,
  type Scope
} from '@algohub/auth-client';
export type { Scope } from '@algohub/auth-client';

export type LearningLanguage = 'c' | 'python';
export type AccessMode = 'auto' | 'unlocked' | 'locked';

export interface LearningQuiz {
  question: string;
  options: string[];
  correct_option?: number;
}

export interface LearningTestCase {
  description: string;
  input: string;
  expected_output: string;
}

export interface LearningTestResult {
  description: string;
  passed: boolean;
  expected_output: string;
  actual_output: string;
}

export interface BrowserTestAttestation {
  lessonId: string;
  lessonVersion: number;
  language: LearningLanguage;
  source: string;
}

export interface UnlockedAchievement {
  achievement_id: string;
  unlocked_at: string;
}

export interface LearningProfile {
  user_id: number;
  user_name: string;
  xp: number;
  level: number;
  next_level_xp: number;
  streak: number;
  last_active_date: string;
  completed_lessons: number;
  games_played: number;
  perfect_games: number;
  prestige: number;
  prestige_count: number;
  unlocked_achievements: UnlockedAchievement[];
  timezone: string;
  timezone_offset: string;
  updated_at: string;
}

export interface LearningPrestigeEntry {
  id: string;
  user_id: number;
  source: 'admin_grant' | 'weekly_leaderboard';
  amount: number;
  reason: string;
  created_at: string;
}

export interface LearningAchievementCatalogItem {
  id: string;
  title: string;
  description: string;
  requirement_type:
    | 'manual'
    | 'xp'
    | 'lesson_count'
    | 'streak'
    | 'game_score'
    | 'level_completed';
  requirement_value: string;
  xp_reward: number;
  active: boolean;
  scope: Scope;
  version: number;
}

export interface LearningAchievementInput {
  title: string;
  description: string;
  requirement_type: LearningAchievementCatalogItem['requirement_type'];
  requirement_value: string;
  xp_reward: number;
  active: boolean;
  scope: Scope;
}

export interface LearningWeeklySnapshot {
  week_start: string;
  generated_at: string;
  rows: LearningLeaderboardRow[];
}

export interface LearningAIHintResponse {
  lesson_id: string;
  hint: string;
  remaining_quota: number;
  simulated: boolean;
}

export interface LearningLesson {
  id: string;
  module_id: string;
  level_id: string;
  title: string;
  sort_order: number;
  language: LearningLanguage;
  explanation: string;
  code_example: string;
  starter_code: string;
  solution?: string;
  hint: string;
  xp_reward: number;
  quiz?: LearningQuiz | null;
  test_cases: LearningTestCase[];
  status?: 'not_started' | 'in_progress' | 'completed';
  scope: Scope;
  version: number;
}

export interface LearningModule {
  id: string;
  level_id: string;
  title: string;
  description: string;
  sort_order: number;
  completed_lessons: number;
  total_lessons: number;
  scope: Scope;
  version: number;
  lessons?: LearningLesson[];
}

export interface LearningLevel {
  id: string;
  title: string;
  description: string;
  sort_order: number;
  access_mode: AccessMode;
  required_xp: number;
  unlocked: boolean;
  completed_lessons: number;
  total_lessons: number;
  scope: Scope;
  version: number;
  created_at: string;
  updated_at: string;
  modules?: LearningModule[];
}

export interface LearningProgress {
  lesson_id: string;
  module_id: string;
  level_id: string;
  user_id: number;
  status: 'in_progress' | 'completed';
  draft_code: string;
  language: LearningLanguage;
  quiz_option: number | null;
  test_passed: boolean;
  revision: number;
  xp_awarded: number;
  completed_at: string | null;
  updated_at: string;
}

export interface LearningAchievement {
  id: string;
  title: string;
  description: string;
  requirement_type:
    'xp' | 'lesson_count' | 'streak' | 'game_score' | 'level_completed';
  requirement_value: string;
  unlocked: boolean;
  unlocked_at: string | null;
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

export interface LearningLevelsView {
  profile: LearningProfile;
  levels: LearningLevel[];
  runner: RunnerInfo;
}

export interface LearningLessonView {
  lesson: LearningLesson;
  progress: LearningProgress;
  profile: LearningProfile;
  newly_unlocked: LearningAchievement[];
}

export interface LearningRunResponse {
  result: RunResult;
  test_results: LearningTestResult[];
  all_passed: boolean;
}

export interface LearningHintResponse {
  lesson_id: string;
  hint: string;
  remaining_quota: number;
  simulated: boolean;
}

export interface LearningAchievementsView {
  profile: LearningProfile;
  achievements: LearningAchievement[];
}

export interface LearningLeaderboardRow {
  rank: number;
  user_id: number;
  user_name: string;
  xp: number;
  level: number;
  streak: number;
  completed_lessons: number;
  achievement_count: number;
}

export interface LearningBugHuntQuestion {
  id: string;
  language: LearningLanguage;
  difficulty: 'easy' | 'medium' | 'hard';
  title: string;
  buggy_code: string;
  bug_line?: number;
  explanation?: string;
  xp_reward: number;
  active: boolean;
  solved?: boolean;
  scope: Scope;
  version?: number;
}

export interface LearningBugHuntAttempt {
  attempt_id: string;
  question_id: string;
  user_id: number;
  language: LearningLanguage;
  selected_line: number;
  explanation: string;
  correct: boolean;
  xp_earned: number;
  feedback: string;
  played_at: string;
}

export interface LearningBugHuntView {
  questions: LearningBugHuntQuestion[];
  attempts: LearningBugHuntAttempt[];
  weekly_used: number;
  weekly_limit: number;
  profile: LearningProfile;
}

export interface LearningBugHuntSubmitView {
  attempt: LearningBugHuntAttempt;
  profile: LearningProfile;
  newly_unlocked: LearningAchievement[];
}

export interface LearningMonitoringRow {
  user_id: number;
  user_name: string;
  xp: number;
  level: number;
  streak: number;
  completed_lessons: number;
  in_progress_lessons: number;
  games_played: number;
  last_active_date: string;
  updated_at: string;
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

export function calculateLevel(xp: number): number {
  if (xp <= 0) return 1;
  return Math.floor(Math.sqrt(xp / 50)) + 1;
}

export function levelProgressPercent(xp: number): number {
  const lvl = calculateLevel(xp);
  const base = 50 * Math.pow(lvl - 1, 2);
  const next = 50 * Math.pow(lvl, 2);
  const span = next - base;
  return span > 0 ? Math.min(100, Math.max(0, ((xp - base) / span) * 100)) : 0;
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

export function errorMessage(error: unknown): string {
  if (!(error instanceof ApiError))
    return 'Koneksi terputus. Draf kode lokal tetap di memori; coba lagi tanpa menutup halaman.';
  const hint =
    error.status === 401
      ? 'Sesi login berakhir. Masuk kembali di tab lain, lalu coba lagi.'
      : error.status === 403
        ? error.name === 'LEVEL_LOCKED'
          ? error.message
          : 'Akses ditolak atau izin berubah. Periksa akun dan scope Anda.'
        : error.status === 409
          ? error.name === 'QUIZ_INCORRECT' ||
            error.name === 'CODE_NOT_VERIFIED' ||
            error.name === 'WEEKLY_LIMIT_REACHED'
            ? error.message
            : 'Data berubah di server. Muat data terbaru dan periksa sebelum mengirim ulang.'
          : error.status === 429
            ? error.message ||
              'Batas permintaan tercapai. Tunggu sebelum mencoba lagi.'
            : error.message;
  return `${hint}${error.requestId ? ` ID permintaan: ${error.requestId}` : ''}`;
}

export class ElearningClient {
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
        'Koneksi E-learning terputus. Coba lagi tanpa menutup halaman.'
      );
    }
    const body = await response.json().catch(() => null);
    if (!response.ok || !body || typeof body !== 'object' || !('data' in body))
      throw new ApiError(
        body?.error?.message ?? 'Layanan E-learning belum tersedia.',
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
  levels(): Promise<LearningLevelsView> {
    return this.get('/learning/levels');
  }
  module(id: string): Promise<LearningModule> {
    return this.get(`/learning/modules/${encodeURIComponent(id)}`);
  }
  lesson(id: string): Promise<LearningLessonView> {
    return this.get(`/learning/lessons/${encodeURIComponent(id)}`);
  }
  leaderboard(offset = 0): Promise<LearningLeaderboardRow[]> {
    return this.get(`/learning/leaderboard?limit=25&offset=${offset}`);
  }
  weeklyStandings(offset = 0): Promise<LearningWeeklySnapshot[]> {
    return this.get(`/learning/leaderboard/weekly?limit=10&offset=${offset}`);
  }
  prestige(userID?: number): Promise<LearningPrestigeEntry[]> {
    return this.get(`/learning/prestige${userID ? `?user_id=${userID}` : ''}`);
  }
  aiHint(command: Command): Promise<LearningAIHintResponse> {
    return this.execute(command);
  }
  achievements(): Promise<LearningAchievementsView> {
    return this.get('/learning/achievements');
  }
  bugHunt(): Promise<LearningBugHuntView> {
    return this.get('/learning/bug-hunt');
  }
}

export interface LessonWorkspaceState {
  view: LearningLessonView | null;
  draftCode: string;
  quizOption: number | null;
  saving: boolean;
  completing: boolean;
  syncing: boolean;
  offline: boolean;
  conflict: boolean;
  dirty: boolean;
  issue: string;
}

export class LessonWorkspace {
  state: LessonWorkspaceState = {
    view: null,
    draftCode: '',
    quizOption: null,
    saving: false,
    completing: false,
    syncing: false,
    offline: false,
    conflict: false,
    dirty: false,
    issue: ''
  };
  private baseCode = '';
  private baseQuiz: number | null = null;
  private pending: Command | null = null;
  private completeCommand: Command | null = null;
  private inFlight: Promise<boolean> | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private disposed = false;
  private client: Pick<ElearningClient, 'lesson' | 'execute'>;
  readonly id: string;
  private changed: (state: LessonWorkspaceState) => void;

  constructor(
    client: Pick<ElearningClient, 'lesson' | 'execute'>,
    id: string,
    changed: (state: LessonWorkspaceState) => void
  ) {
    this.client = client;
    this.id = id;
    this.changed = changed;
  }

  private emit() {
    if (this.disposed) return;
    this.state.dirty =
      this.state.draftCode !== this.baseCode ||
      this.state.quizOption !== this.baseQuiz;
    this.changed({ ...this.state });
  }

  private adopt(
    incoming: LearningLessonView,
    sent?: { draft_code: string; quiz_option: number | null },
    forceConflict = false
  ) {
    if (
      this.state.view &&
      (incoming.lesson.id !== this.state.view.lesson.id ||
        incoming.progress.revision < this.state.view.progress.revision)
    )
      return;
    let prevCode = this.baseCode;
    let prevQuiz = this.baseQuiz;
    if (!this.state.view) {
      this.state.draftCode = incoming.progress.draft_code;
      this.state.quizOption = incoming.progress.quiz_option;
      prevCode = incoming.progress.draft_code;
      prevQuiz = incoming.progress.quiz_option;
    } else if (sent) {
      if (incoming.progress.draft_code === sent.draft_code)
        prevCode = sent.draft_code;
      if (incoming.progress.quiz_option === sent.quiz_option)
        prevQuiz = sent.quiz_option;
    }
    const localCodeChanged = this.state.draftCode !== prevCode;
    const localQuizChanged = this.state.quizOption !== prevQuiz;
    const remoteCodeChanged = incoming.progress.draft_code !== prevCode;
    const remoteQuizChanged = incoming.progress.quiz_option !== prevQuiz;

    const codeConflict =
      localCodeChanged &&
      remoteCodeChanged &&
      this.state.draftCode !== incoming.progress.draft_code;
    const quizConflict =
      localQuizChanged &&
      remoteQuizChanged &&
      this.state.quizOption !== incoming.progress.quiz_option;

    if (!localCodeChanged) this.state.draftCode = incoming.progress.draft_code;
    if (!localQuizChanged)
      this.state.quizOption = incoming.progress.quiz_option;

    this.baseCode = incoming.progress.draft_code;
    this.baseQuiz = incoming.progress.quiz_option;
    this.state.view = incoming;
    this.state.conflict =
      codeConflict ||
      quizConflict ||
      (forceConflict &&
        (this.state.draftCode !== this.baseCode ||
          this.state.quizOption !== this.baseQuiz));
    this.emit();
  }

  async refresh(forceConflict = false): Promise<boolean> {
    if (this.state.syncing) return false;
    this.state.syncing = true;
    this.emit();
    try {
      const view = await this.client.lesson(this.id);
      const sent = this.pending
        ? (JSON.parse(this.pending.body) as {
            draft_code: string;
            quiz_option: number | null;
          })
        : undefined;
      this.state.issue = '';
      this.adopt(view, sent, forceConflict);
      return true;
    } catch (error) {
      this.state.issue = errorMessage(error);
      return false;
    } finally {
      this.state.syncing = false;
      this.emit();
    }
  }

  editCode(value: string) {
    if (!this.state.view || this.state.conflict) return;
    if (new TextEncoder().encode(value).length > 64 * 1024) {
      this.state.issue = 'Kode maksimal 64 KiB.';
      this.emit();
      return;
    }
    this.state.draftCode = value;
    this.state.issue = '';
    this.emit();
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.flush(), 700);
  }

  selectQuiz(option: number | null) {
    if (!this.state.view || this.state.conflict) return;
    this.state.quizOption = option;
    this.state.issue = '';
    this.emit();
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.flush(), 700);
  }

  setOffline(offline: boolean) {
    this.state.offline = offline;
    this.emit();
  }

  async reconnect() {
    this.setOffline(false);
    if (await this.refresh()) {
      if (!this.state.conflict) await this.flush();
    }
  }

  resolve(useLocal: boolean) {
    this.pending = null;
    if (!useLocal) {
      this.state.draftCode = this.baseCode;
      this.state.quizOption = this.baseQuiz;
    }
    this.state.conflict = false;
    this.state.issue = '';
    this.emit();
    if (useLocal) void this.flush();
  }

  async flush(): Promise<boolean> {
    clearTimeout(this.timer);
    if (this.inFlight) {
      const ok = await this.inFlight;
      return ok ? this.flush() : false;
    }
    if (this.state.offline || this.state.conflict || !this.state.view)
      return false;
    if (!this.pending && !this.state.dirty) return true;
    if (!this.pending) {
      this.pending = command(
        `/learning/lessons/${encodeURIComponent(this.id)}/progress`,
        'PUT',
        {
          revision: this.state.view.progress.revision,
          draft_code: this.state.draftCode,
          language: this.state.view.lesson.language,
          quiz_option: this.state.quizOption,
          complete: false
        }
      );
    }
    const pending = this.pending;
    this.state.saving = true;
    this.emit();
    this.inFlight = (async () => {
      try {
        const result = await this.client.execute<LearningLessonView>(pending);
        const sent = JSON.parse(pending.body) as {
          draft_code: string;
          quiz_option: number | null;
        };
        this.pending = null;
        this.state.issue = '';
        this.adopt(result, sent);
        return !this.state.conflict;
      } catch (error) {
        this.state.issue = errorMessage(error);
        if (error instanceof ApiError && error.status === 409) {
          this.pending = null;
          await this.refresh(true);
        } else if (
          error instanceof ApiError &&
          error.status >= 400 &&
          error.status < 500 &&
          ![401, 403, 408, 429].includes(error.status)
        ) {
          this.pending = null;
        }
        return false;
      } finally {
        this.state.saving = false;
        this.emit();
      }
    })();
    let ok: boolean;
    try {
      ok = await this.inFlight;
    } finally {
      this.inFlight = null;
    }
    return ok && this.state.dirty ? this.flush() : ok;
  }

  async complete(
    attestation?: BrowserTestAttestation
  ): Promise<LearningLessonView | null> {
    if (
      this.state.completing ||
      this.state.offline ||
      this.state.conflict ||
      !this.state.view
    )
      return null;
    const lesson = this.state.view.lesson;
    const codeRequired = Boolean(
      lesson.starter_code || lesson.test_cases.length
    );
    if (
      codeRequired &&
      (!attestation ||
        attestation.lessonId !== lesson.id ||
        attestation.lessonVersion !== lesson.version ||
        attestation.language !== lesson.language ||
        attestation.source !== this.state.draftCode)
    ) {
      this.state.issue =
        'Jalankan kode saat ini di browser sampai seluruh testcase lulus.';
      this.emit();
      return null;
    }
    this.state.completing = true;
    this.emit();
    try {
      const saved = await this.flush();
      if (!saved || !this.state.view) return null;
      const body = {
        revision: this.state.view.progress.revision,
        draft_code: this.state.draftCode,
        language: this.state.view.lesson.language,
        quiz_option: this.state.quizOption,
        complete: true,
        browser_test_passed: codeRequired,
        lesson_version: codeRequired ? lesson.version : 0
      };
      this.completeCommand = reusableCommand(
        this.completeCommand,
        `/learning/lessons/${encodeURIComponent(this.id)}/progress`,
        'PUT',
        body
      );
      const result = await this.client.execute<LearningLessonView>(
        this.completeCommand
      );
      this.completeCommand = null;
      this.state.issue = '';
      this.adopt(result, {
        draft_code: body.draft_code,
        quiz_option: body.quiz_option
      });
      return result;
    } catch (error) {
      this.state.issue = errorMessage(error);
      if (error instanceof ApiError && error.status === 409) {
        this.completeCommand = null;
        if (!['QUIZ_INCORRECT', 'CODE_NOT_VERIFIED'].includes(error.name))
          await this.refresh(true);
      }
      return null;
    } finally {
      this.state.completing = false;
      this.emit();
    }
  }

  dispose() {
    this.disposed = true;
    clearTimeout(this.timer);
  }
}
