import { ApiError } from '@algohub/auth-client';
import {
  command,
  errorMessage,
  type Answer,
  type Attempt,
  type Command,
  type PracticumClient
} from './index.ts';

export class ServerClock {
  private anchor = 0;
  private remaining = 0;
  private deadline = '';
  private now: () => number;
  constructor(now: () => number = () => performance.now()) {
    this.now = now;
  }
  sync(attempt: Pick<Attempt, 'server_now' | 'deadline_at'>) {
    const delta =
      Date.parse(attempt.deadline_at) - Date.parse(attempt.server_now);
    const duration = Number.isFinite(delta) ? Math.max(0, delta) : 0;
    this.remaining =
      this.deadline === attempt.deadline_at
        ? Math.min(this.milliseconds(), duration)
        : duration;
    this.deadline = attempt.deadline_at;
    this.anchor = this.now();
  }
  milliseconds(): number {
    return Math.max(0, this.remaining - Math.max(0, this.now() - this.anchor));
  }
}
export function timerLabel(milliseconds: number): string {
  const seconds = Math.ceil(milliseconds / 1000);
  return `${Math.floor(seconds / 60)
    .toString()
    .padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`;
}
export function answersById(attempt: Attempt): Record<string, Answer> {
  return Object.fromEntries(
    attempt.questions.map((question) => [
      question.id,
      {
        question_id: question.id,
        value: '',
        language: question.language || undefined,
        ...attempt.answers?.find((answer) => answer.question_id === question.id)
      }
    ])
  );
}
export function sameAnswer(
  a: Answer | undefined,
  b: Answer | undefined
): boolean {
  return (
    (a?.value ?? '') === (b?.value ?? '') &&
    (a?.language ?? '') === (b?.language ?? '')
  );
}
export function acceptsAttempt(
  current: Attempt | null,
  incoming: Attempt
): boolean {
  const rank = { in_progress: 0, submitted: 1, graded: 2 };
  return (
    !current ||
    (incoming.attempt_id === current.attempt_id &&
      incoming.revision >= current.revision &&
      incoming.grade_revision >= current.grade_revision &&
      rank[incoming.status] >= rank[current.status])
  );
}
export function reconcileDrafts(
  base: Record<string, Answer>,
  local: Record<string, Answer>,
  incoming: Record<string, Answer>
) {
  const drafts = { ...incoming };
  const conflicts: string[] = [];
  for (const [id, answer] of Object.entries(local)) {
    if (sameAnswer(answer, base[id])) continue;
    drafts[id] = answer;
    if (
      !sameAnswer(incoming[id], base[id]) &&
      !sameAnswer(incoming[id], answer)
    )
      conflicts.push(id);
  }
  return { drafts, conflicts };
}
export interface WorkspaceState {
  attempt: Attempt | null;
  drafts: Record<string, Answer>;
  saving: boolean;
  submitting: boolean;
  syncing: boolean;
  offline: boolean;
  blocked: boolean;
  conflict: boolean;
  dirty: boolean;
  issue: string;
}
export class Workspace {
  readonly clock: ServerClock;
  state: WorkspaceState = {
    attempt: null,
    drafts: {},
    saving: false,
    submitting: false,
    syncing: false,
    offline: false,
    blocked: false,
    conflict: false,
    dirty: false,
    issue: ''
  };
  private base: Record<string, Answer> = {};
  private pending: Command | null = null;
  private submitCommand: Command | null = null;
  private inFlight: Promise<boolean> | null = null;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private disposed = false;
  private refreshFailures = 0;
  private client: Pick<PracticumClient, 'attempt' | 'execute'>;
  readonly id: string;
  private changed: (state: WorkspaceState) => void;
  constructor(
    client: Pick<PracticumClient, 'attempt' | 'execute'>,
    id: string,
    changed: (state: WorkspaceState) => void,
    now?: () => number
  ) {
    this.client = client;
    this.id = id;
    this.changed = changed;
    this.clock = new ServerClock(now);
  }
  private emit() {
    if (this.disposed) return;
    this.state.dirty = Object.entries(this.state.drafts).some(
      ([id, answer]) => !sameAnswer(answer, this.base[id])
    );
    this.changed({ ...this.state, drafts: { ...this.state.drafts } });
  }
  private adopt(
    attempt: Attempt,
    sent?: Record<string, Answer>,
    forceConflict = false
  ) {
    if (!acceptsAttempt(this.state.attempt, attempt)) return;
    const incoming = answersById(attempt);
    const previousBase = { ...this.base };
    // Acknowledged bytes become the baseline, not edits made while saving.
    if (sent)
      for (const [id, answer] of Object.entries(sent))
        if (sameAnswer(incoming[id], answer)) previousBase[id] = answer;
    const result = reconcileDrafts(previousBase, this.state.drafts, incoming);
    this.state.drafts = result.drafts;
    this.base = incoming;
    this.state.attempt = attempt;
    this.state.conflict =
      result.conflicts.length > 0 ||
      (forceConflict &&
        Object.entries(result.drafts).some(
          ([id, answer]) => !sameAnswer(answer, incoming[id])
        ));
    this.clock.sync(attempt);
    if (
      attempt.status !== 'in_progress' &&
      Object.entries(result.drafts).some(
        ([id, answer]) => !sameAnswer(answer, incoming[id])
      )
    )
      this.state.issue =
        'Attempt ditutup. Perubahan lokal yang belum tersimpan tidak masuk pengumpulan; salin secara manual bila diperlukan.';
    this.emit();
  }
  async poll(): Promise<boolean> {
    if (
      this.disposed ||
      this.refreshFailures >= 3 ||
      this.state.offline ||
      this.state.blocked ||
      this.state.conflict ||
      this.state.saving ||
      this.state.submitting ||
      (this.state.attempt && this.state.attempt.status !== 'in_progress')
    )
      return false;
    return this.refresh();
  }
  async refresh(forceConflict = false): Promise<boolean> {
    if (this.disposed || this.state.syncing) return false;
    this.state.syncing = true;
    this.emit();
    try {
      const attempt = await this.client.attempt(this.id);
      const sent = this.pending
        ? Object.fromEntries(
            (JSON.parse(this.pending.body).answers as Answer[]).map((a) => [
              a.question_id,
              a
            ])
          )
        : undefined;
      this.refreshFailures = 0;
      this.state.blocked = false;
      this.state.issue = '';
      this.adopt(attempt, sent, forceConflict);
      return true;
    } catch (error) {
      this.refreshFailures += 1;
      this.failure(error);
      return false;
    } finally {
      this.state.syncing = false;
      this.emit();
    }
  }
  edit(questionId: string, value: string, language?: string) {
    if (
      !this.editable() ||
      !this.state.attempt?.questions.some(
        (question) => question.id === questionId
      )
    )
      return;
    if (new TextEncoder().encode(value).length > 64 * 1024) {
      this.state.issue = 'Jawaban maksimal 64 KiB.';
      this.emit();
      return;
    }
    this.state.drafts[questionId] = {
      question_id: questionId,
      value,
      language
    };
    this.state.issue = '';
    this.emit();
    clearTimeout(this.timer);
    this.timer = setTimeout(() => void this.flush(), 700);
  }
  editable(): boolean {
    return (
      this.state.attempt?.status === 'in_progress' &&
      this.clock.milliseconds() > 0 &&
      !this.state.submitting &&
      !this.state.blocked &&
      !this.state.conflict
    );
  }
  setOffline(offline: boolean) {
    this.state.offline = offline;
    this.emit();
  }
  private failure(error: unknown) {
    this.state.issue = errorMessage(error);
    if (error instanceof ApiError && [401, 403].includes(error.status))
      this.state.blocked = true;
    this.emit();
  }
  async reconnect() {
    this.setOffline(false);
    if (await this.refresh()) {
      if (this.state.attempt?.status !== 'in_progress') return;
      if (this.clock.milliseconds() <= 0) await this.submit();
      else if (!this.state.conflict) await this.flush();
    }
  }
  resolve(useLocal: boolean) {
    this.pending = null;
    if (!useLocal) this.state.drafts = { ...this.base };
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
    if (this.state.offline || this.state.blocked || this.state.conflict)
      return false;
    if (this.state.attempt?.status !== 'in_progress') return !this.state.dirty;
    if (!this.pending && !this.state.dirty) return true;
    if (this.clock.milliseconds() <= 0) return false;
    if (!this.pending) {
      const answers = Object.values(this.state.drafts)
        .filter((a) => !sameAnswer(a, this.base[a.question_id]))
        .map(({ question_id, value, language }) => ({
          question_id,
          value,
          language
        }));
      this.pending = command(
        `/practicum/attempts/${encodeURIComponent(this.id)}/answers`,
        'PATCH',
        { revision: this.state.attempt.revision, answers }
      );
      if (new TextEncoder().encode(this.pending.body).length > 256 * 1024) {
        this.pending = null;
        this.state.issue =
          'Total jawaban melampaui 256 KiB. Kurangi isi sebelum menyimpan.';
        this.emit();
        return false;
      }
    }
    const pending = this.pending;
    this.state.saving = true;
    this.emit();
    this.inFlight = (async () => {
      try {
        const result = await this.client.execute<Attempt>(pending);
        const sent = Object.fromEntries(
          (JSON.parse(pending.body).answers as Answer[]).map((a) => [
            a.question_id,
            a
          ])
        );
        this.pending = null;
        this.state.issue = '';
        this.adopt(result, sent);
        return !this.state.conflict;
      } catch (error) {
        this.failure(error);
        if (error instanceof ApiError && error.status === 409) {
          this.pending = null;
          await this.refresh(true);
        } else if (
          error instanceof ApiError &&
          error.status >= 400 &&
          error.status < 500 &&
          ![401, 403, 408, 429].includes(error.status)
        )
          this.pending = null;
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
    return ok &&
      this.state.dirty &&
      this.state.attempt?.status === 'in_progress'
      ? this.flush()
      : ok;
  }
  async submit(): Promise<boolean> {
    if (
      this.state.submitting ||
      this.state.offline ||
      this.state.blocked ||
      (this.state.conflict && this.clock.milliseconds() > 0) ||
      this.state.attempt?.status !== 'in_progress'
    )
      return false;
    this.state.submitting = true;
    this.emit();
    try {
      const saved = await this.flush();
      if (!saved && this.clock.milliseconds() > 0) return false;
      // After expiry only the server's last accepted answers can be submitted.
      if (this.state.attempt?.status !== 'in_progress') return true;
      this.submitCommand ??= command(
        `/practicum/attempts/${encodeURIComponent(this.id)}/submit`,
        'POST',
        {}
      );
      const result = await this.client.execute<Attempt>(this.submitCommand);
      this.adopt(result);
      return result.status !== 'in_progress';
    } catch (error) {
      this.failure(error);
      if (error instanceof ApiError && error.status === 409)
        await this.refresh(true);
      return false;
    } finally {
      this.state.submitting = false;
      this.emit();
    }
  }
  dispose() {
    this.disposed = true;
    clearTimeout(this.timer);
  }
}
