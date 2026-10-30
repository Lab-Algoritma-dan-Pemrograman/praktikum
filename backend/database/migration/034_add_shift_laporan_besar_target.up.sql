-- 034: dukung window per shift pada laporan_besar_target.
-- shift NULL = berlaku untuk SEMUA shift (fallback), shift 1/2 = spesifik.
ALTER TABLE laporan_besar_target ADD COLUMN IF NOT EXISTS shift INT NULL;

ALTER TABLE laporan_besar_target DROP CONSTRAINT IF EXISTS uq_lapbes_target;
ALTER TABLE laporan_besar_target ADD CONSTRAINT uq_lapbes_target UNIQUE (laporan_besar_id, kelas_id, shift);
