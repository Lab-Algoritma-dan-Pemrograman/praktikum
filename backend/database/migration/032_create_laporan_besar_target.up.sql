CREATE TABLE IF NOT EXISTS laporan_besar_target (
    id               SERIAL PRIMARY KEY,
    laporan_besar_id INT       NOT NULL,
    kelas_id         INT       NOT NULL,
    buka_at          TIMESTAMP NULL,
    deadline_at      TIMESTAMP NOT NULL,
    late_until       TIMESTAMP NULL,
    CONSTRAINT uq_lapbes_target UNIQUE (laporan_besar_id, kelas_id),
    CONSTRAINT fk_lbt_lapbes FOREIGN KEY (laporan_besar_id) REFERENCES laporan_besar(id) ON DELETE CASCADE,
    CONSTRAINT fk_lbt_kelas FOREIGN KEY (kelas_id) REFERENCES kelas(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_lbt_kelas ON laporan_besar_target(kelas_id);
