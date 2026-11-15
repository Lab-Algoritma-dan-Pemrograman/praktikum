import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  attemptLabels,
  gradingLabels,
  questionTypeLabels,
  difficultyLabels,
  runLabels,
  submissionLabel
} from '../src/index.ts';

test('Praktikum display labels distinguish work, publication and non-authoritative preview', () => {
  assert.equal(attemptLabels.in_progress, 'Sedang dikerjakan');
  assert.equal(gradingLabels.pending, 'Menunggu penilaian');
  assert.equal(attemptLabels.graded, 'Nilai dipublikasikan');
  assert.equal(questionTypeLabels.essay, 'Uraian');
  assert.equal(difficultyLabels.medium, 'Menengah');
  assert.equal(runLabels.success, 'Berhasil');
  assert.equal(
    submissionLabel('legacy_submit'),
    'riwayat pengumpulan sebelumnya'
  );
  assert.equal(submissionLabel(null), 'pengumpulan selesai');
  assert.equal(
    submissionLabel('unknown-sensitive-value'),
    'pengumpulan selesai'
  );
});
