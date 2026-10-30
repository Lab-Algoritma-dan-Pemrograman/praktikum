ALTER TABLE laporan_besar_target DROP CONSTRAINT IF EXISTS uq_lapbes_target;
ALTER TABLE laporan_besar_target ADD CONSTRAINT uq_lapbes_target UNIQUE (laporan_besar_id, kelas_id);

ALTER TABLE laporan_besar_target DROP COLUMN IF EXISTS shift;
