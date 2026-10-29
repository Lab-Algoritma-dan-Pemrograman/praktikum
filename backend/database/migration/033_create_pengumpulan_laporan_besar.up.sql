CREATE TABLE IF NOT EXISTS pengumpulan_laporan_besar (
    id               SERIAL PRIMARY KEY,
    laporan_besar_id INT          NOT NULL,
    mahasiswa_id     INT          NOT NULL,
    status           VARCHAR(20)  NOT NULL DEFAULT 'belum',
    sumber           VARCHAR(10)  NULL,
    laporan_url      VARCHAR(500) NULL,
    laporan_path     VARCHAR(300) NULL,
    analisa_url      VARCHAR(500) NULL,
    analisa_path     VARCHAR(300) NULL,
    submitted_at     TIMESTAMP    NULL,
    is_late          BOOLEAN      NOT NULL DEFAULT FALSE,
    manual_oleh      INT          NULL,
    manual_pada      TIMESTAMP    NULL,
    manual_catatan   VARCHAR(300) NULL,
    override_oleh    INT          NULL,
    override_alasan  VARCHAR(500) NULL,
    override_pada    TIMESTAMP    NULL,
    CONSTRAINT uq_plb UNIQUE (laporan_besar_id, mahasiswa_id),
    CONSTRAINT fk_plb_lapbes FOREIGN KEY (laporan_besar_id) REFERENCES laporan_besar(id) ON DELETE CASCADE,
    CONSTRAINT fk_plb_mhs FOREIGN KEY (mahasiswa_id) REFERENCES users(id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_plb_mhs ON pengumpulan_laporan_besar(mahasiswa_id);
