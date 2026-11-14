export type SesiJadwal = {
  id: string;
  pekan: number;
  hari: number; // 1 = Senin ... 7 = Minggu
  mulai: string; // "08:00"
  selesai: string; // "10:00"
  kelas: string;
  kode: string;
  ruang: string;
  asisten: string;
  modul: string;
};

export const HARI = [
  'Senin',
  'Selasa',
  'Rabu',
  'Kamis',
  'Jumat',
  'Sabtu',
  'Minggu'
] as const;

export const JAM_MULAI = 8;
export const JAM_SELESAI = 18;
export const TINGGI_BARIS = 52;

export const pekanList = [
  { ke: 0, label: '10 – 16 Feb 2025', tanggal: [10, 11, 12, 13, 14, 15, 16] },
  { ke: 1, label: '17 – 23 Feb 2025', tanggal: [17, 18, 19, 20, 21, 22, 23] },
  { ke: 2, label: '24 Feb – 2 Mar 2025', tanggal: [24, 25, 26, 27, 28, 1, 2] }
];

export const sesiJadwal: SesiJadwal[] = [
  // pekan 0
  {
    id: 's1',
    pekan: 0,
    hari: 1,
    mulai: '08:00',
    selesai: '10:00',
    kelas: 'Algoritma & Pemrograman',
    kode: 'IF-2A',
    ruang: 'Lab Dasar 1',
    asisten: 'Rani',
    modul: 'Modul 3 — Sorting'
  },
  {
    id: 's2',
    pekan: 0,
    hari: 1,
    mulai: '13:00',
    selesai: '15:00',
    kelas: 'Struktur Data',
    kode: 'IF-3B',
    ruang: 'Lab Dasar 2',
    asisten: 'Dimas',
    modul: 'Modul 5 — Graph'
  },
  {
    id: 's3',
    pekan: 0,
    hari: 2,
    mulai: '10:00',
    selesai: '12:00',
    kelas: 'Basis Data',
    kode: 'IF-2C',
    ruang: 'Lab Basis Data',
    asisten: 'Nadia',
    modul: 'Modul 4 — Join'
  },
  {
    id: 's4',
    pekan: 0,
    hari: 3,
    mulai: '08:00',
    selesai: '11:00',
    kelas: 'Pemrograman Web',
    kode: 'IF-3A',
    ruang: 'Lab Rekayasa',
    asisten: 'Fajar',
    modul: 'Modul 6 — REST'
  },
  {
    id: 's5',
    pekan: 0,
    hari: 3,
    mulai: '15:00',
    selesai: '17:00',
    kelas: 'Sistem Operasi',
    kode: 'IF-3C',
    ruang: 'Lab Sistem',
    asisten: 'Sari',
    modul: 'Modul 2 — Process'
  },
  {
    id: 's6',
    pekan: 0,
    hari: 4,
    mulai: '09:00',
    selesai: '11:00',
    kelas: 'Jaringan Komputer',
    kode: 'IF-4A',
    ruang: 'Lab Jaringan',
    asisten: 'Bagas',
    modul: 'Modul 3 — Subnetting'
  },
  {
    id: 's7',
    pekan: 0,
    hari: 5,
    mulai: '13:00',
    selesai: '15:00',
    kelas: 'Kecerdasan Buatan',
    kode: 'IF-4B',
    ruang: 'Lab AI',
    asisten: 'Intan',
    modul: 'Modul 2 — Regresi'
  },
  {
    id: 's8',
    pekan: 0,
    hari: 6,
    mulai: '09:00',
    selesai: '12:00',
    kelas: 'Proyek Capstone',
    kode: 'IF-4C',
    ruang: 'Lab Kolaborasi',
    asisten: 'Rani',
    modul: 'Sprint review'
  },

  // pekan 1
  {
    id: 's9',
    pekan: 1,
    hari: 1,
    mulai: '08:00',
    selesai: '10:00',
    kelas: 'Algoritma & Pemrograman',
    kode: 'IF-2A',
    ruang: 'Lab Dasar 1',
    asisten: 'Rani',
    modul: 'Modul 4 — Searching'
  },
  {
    id: 's10',
    pekan: 1,
    hari: 1,
    mulai: '13:00',
    selesai: '15:00',
    kelas: 'Struktur Data',
    kode: 'IF-3B',
    ruang: 'Lab Dasar 2',
    asisten: 'Dimas',
    modul: 'Modul 6 — Hashing'
  },
  {
    id: 's11',
    pekan: 1,
    hari: 2,
    mulai: '10:00',
    selesai: '12:00',
    kelas: 'Basis Data',
    kode: 'IF-2C',
    ruang: 'Lab Basis Data',
    asisten: 'Nadia',
    modul: 'Modul 5 — Index'
  },
  {
    id: 's12',
    pekan: 1,
    hari: 3,
    mulai: '08:00',
    selesai: '11:00',
    kelas: 'Pemrograman Web',
    kode: 'IF-3A',
    ruang: 'Lab Rekayasa',
    asisten: 'Fajar',
    modul: 'Modul 7 — Auth'
  },
  {
    id: 's13',
    pekan: 1,
    hari: 4,
    mulai: '09:00',
    selesai: '11:00',
    kelas: 'Jaringan Komputer',
    kode: 'IF-4A',
    ruang: 'Lab Jaringan',
    asisten: 'Bagas',
    modul: 'Modul 4 — Routing'
  },
  {
    id: 's14',
    pekan: 1,
    hari: 5,
    mulai: '10:00',
    selesai: '12:00',
    kelas: 'Kecerdasan Buatan',
    kode: 'IF-4B',
    ruang: 'Lab AI',
    asisten: 'Intan',
    modul: 'Modul 3 — Klasifikasi'
  },
  {
    id: 's15',
    pekan: 1,
    hari: 6,
    mulai: '09:00',
    selesai: '12:00',
    kelas: 'Proyek Capstone',
    kode: 'IF-4C',
    ruang: 'Lab Kolaborasi',
    asisten: 'Sari',
    modul: 'Demo hari ke-1'
  },

  // pekan 2
  {
    id: 's16',
    pekan: 2,
    hari: 1,
    mulai: '09:00',
    selesai: '11:00',
    kelas: 'Algoritma & Pemrograman',
    kode: 'IF-2A',
    ruang: 'Lab Dasar 1',
    asisten: 'Rani',
    modul: 'Kuis tengah semester'
  },
  {
    id: 's17',
    pekan: 2,
    hari: 2,
    mulai: '13:00',
    selesai: '15:00',
    kelas: 'Basis Data',
    kode: 'IF-2C',
    ruang: 'Lab Basis Data',
    asisten: 'Nadia',
    modul: 'Modul 6 — Normalisasi'
  },
  {
    id: 's18',
    pekan: 2,
    hari: 3,
    mulai: '10:00',
    selesai: '12:00',
    kelas: 'Sistem Operasi',
    kode: 'IF-3C',
    ruang: 'Lab Sistem',
    asisten: 'Sari',
    modul: 'Modul 3 — Scheduler'
  },
  {
    id: 's19',
    pekan: 2,
    hari: 4,
    mulai: '08:00',
    selesai: '10:00',
    kelas: 'Struktur Data',
    kode: 'IF-3B',
    ruang: 'Lab Dasar 2',
    asisten: 'Dimas',
    modul: 'Modul 7 — Tree'
  },
  {
    id: 's20',
    pekan: 2,
    hari: 5,
    mulai: '09:00',
    selesai: '11:00',
    kelas: 'Pemrograman Web',
    kode: 'IF-3A',
    ruang: 'Lab Rekayasa',
    asisten: 'Fajar',
    modul: 'Modul 8 — Testing'
  },
  {
    id: 's21',
    pekan: 2,
    hari: 6,
    mulai: '10:00',
    selesai: '12:00',
    kelas: 'Jaringan Komputer',
    kode: 'IF-4A',
    ruang: 'Lab Jaringan',
    asisten: 'Bagas',
    modul: 'Modul 5 — Firewall'
  }
];

export function menit(jam: string): number {
  const [h, m] = jam.split(':').map(Number);
  return h * 60 + m;
}

export function sesiPerPekan(pekan: number): SesiJadwal[] {
  return sesiJadwal.filter((s) => s.pekan === pekan);
}
