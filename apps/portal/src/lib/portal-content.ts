/**
 * Public, safe-by-default portal content. No names, contacts, grades or
 * personal schedules: division codes are published, people are not.
 */
export const divisions = [
  {
    code: 'sekretaris',
    focus: 'Administrasi sesi, surat, dan arsip praktikum.'
  },
  {
    code: 'bendahara',
    focus: 'Keuangan operasional laboratorium dan penggantian biaya.'
  },
  {
    code: 'k3',
    focus: 'Keselamatan dan kesehatan kerja selama kegiatan di lab.'
  },
  {
    code: 'labops',
    focus: 'Operasional lab, inventaris, dan penyewaan alat.'
  },
  {
    code: 'pendidikan',
    focus: 'Akademik praktikum dan pembinaan asisten.'
  },
  {
    code: 'pdd',
    focus: 'Dokumentasi dan pengembangan internal laboratorium.'
  }
] as const;

export const weeklyPhases = [
  {
    step: '01',
    title: 'Aktivasi sesi',
    body: 'Asisten membuka sesi untuk kelas atau shift yang terjadwal, termasuk token presensi dan batas waktu pengerjaan.'
  },
  {
    step: '02',
    title: 'Pengerjaan',
    body: 'Peserta mengerjakan soal di aplikasi Praktikum. Timer berjalan di server, jawaban tersimpan otomatis, dan submission ditutup saat waktu habis.'
  },
  {
    step: '03',
    title: 'Peninjauan',
    body: 'Submission masuk ke antrean penilaian. Nilai diputuskan melalui review manual, bukan dari hasil eksekusi kode di browser.'
  },
  {
    step: '04',
    title: 'Publikasi',
    body: 'Nilai dan umpan balik tampil setelah asisten atau koordinator mempublikasikannya. Sebelum itu, statusnya masih tertunda.'
  }
] as const;

export const scheduleNotes = [
  'Jadwal personal, jadwal jaga asisten, dan token sesi hanya terlihat setelah masuk dengan akun Algohub.',
  'Kalender publik tidak memuat kelas, ruang, atau nama peserta, karena data itu terikat scope akademik.',
  'Perubahan jadwal diumumkan melalui kanal resmi laboratorium dan halaman sesi di aplikasi Praktikum.'
] as const;

export const reportSections = [
  {
    title: 'Identitas',
    body: 'Judul praktikum, nama modul, kelas, dan tanggal pelaksanaan.'
  },
  {
    title: 'Tujuan',
    body: 'Hasil belajar yang ingin dicapai pada modul tersebut.'
  },
  {
    title: 'Dasar teori',
    body: 'Ringkasan konsep yang relevan, ditulis ulang dan bukan disalin.'
  },
  {
    title: 'Alat dan bahan',
    body: 'Perangkat, bahasa, dan pustaka yang dipakai selama praktikum.'
  },
  {
    title: 'Langkah kerja',
    body: 'Urutan percobaan yang benar-benar dijalankan, termasuk perubahan yang dilakukan.'
  },
  {
    title: 'Data dan hasil',
    body: 'Keluaran program, tangkapan layar, atau tabel pengukuran apa adanya.'
  },
  {
    title: 'Analisis',
    body: 'Penjelasan mengapa hasilnya demikian, termasuk error dan keterbatasannya.'
  },
  {
    title: 'Kesimpulan',
    body: 'Jawaban atas tujuan praktikum, bukan ringkasan ulang langkah.'
  },
  {
    title: 'Lampiran',
    body: 'Kode sumber, berkas pendukung, dan bukti tambahan yang diminta asisten.'
  }
] as const;

export const reportRules = [
  'Laporan dikumpulkan melalui aplikasi Praktikum pada sesi yang sesuai, bukan lewat pesan pribadi.',
  'Batas waktu mengikuti sesi. Submission setelah tenggat hanya diterima bila sesi masih dibuka.',
  'Nilai berasal dari review manual. Menjalankan kode di browser tidak menambah nilai.',
  'Data, keluaran, dan tangkapan layar harus berasal dari percobaan sendiri; kutipan wajib diberi sumber.'
] as const;

export const modulePacks = [
  {
    name: 'Job sheet',
    body: 'Lembar kerja per modul: tujuan, alat dan bahan, langkah, serta pertanyaan yang harus dijawab.'
  },
  {
    name: 'Pedoman praktikum',
    body: 'Aturan pelaksanaan, keselamatan kerja, presensi, tenggat, dan tata tertib laboratorium.'
  },
  {
    name: 'Template laporan',
    body: 'Kerangka laporan resmi agar struktur pengumpulan konsisten antar kelas.'
  },
  {
    name: 'Materi pendukung',
    body: 'Rujukan singkat, contoh kode, dan bahan pengayaan yang dipakai asisten saat sesi.'
  }
] as const;

export const moduleAccess = [
  'Berkas modul dibagikan melalui aplikasi E-learning dan sesi Praktikum yang sedang berjalan.',
  'Halaman ini hanya memuat jenis dokumen dan cara mengaksesnya, bukan salinan berkas internal.',
  'Modul untuk kelas yang belum aktif tidak ditampilkan sebelum sesinya dibuka.'
] as const;
