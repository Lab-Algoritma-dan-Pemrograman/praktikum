CREATE TABLE IF NOT EXISTS laporan_besar (
    id            SERIAL PRIMARY KEY,
    judul         VARCHAR(200) NOT NULL DEFAULT 'Laporan Besar',
    deskripsi     TEXT,
    is_aktif      BOOLEAN      NOT NULL DEFAULT TRUE,
    butuh_laporan BOOLEAN      NOT NULL DEFAULT TRUE,
    butuh_analisa BOOLEAN      NOT NULL DEFAULT TRUE,
    created_by    INT          NULL,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NULL,
    CONSTRAINT fk_lapbes_creator FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);
