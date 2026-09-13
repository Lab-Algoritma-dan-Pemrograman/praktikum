export type Role = 'asisten' | 'mahasiswa' | 'koordinator';

export interface User {
	id: number;
	role: Role;
	nim: string;
	nama: string;
	is_registered?: boolean;
	kelas_id?: number | null;
	nama_kelas?: string;
	shift?: number | null;
	gelombang?: number | null;
	foto_url?: string | null;
	nomor_hp?: string | null;
	medsos_link?: string | null;
	kelompok?: string | null;
	kelas?: Kelas;
}

export interface AmpuanKelompok {
	id: number;
	asisten_id: number;
	kelas_id: number;
	kelompok: string;
	asisten?: User;
	kelas?: Kelas;
}

export interface AuthResponse {
	token: string;
	user: User;
}

export interface CekNIMResponse {
	nim: string;
	ditemukan: boolean;
	is_registered: boolean;
	nama?: string;
	pesan: string;
}

export interface Kelas {
	id: number;
	nama_kelas: string;
}

export interface Jadwal {
	id: number;
	kelas_id: number;
	shift: number;
	hari: string;
	jam_mulai: string;
	jam_selesai: string;
	keterangan: string;
	kelas?: Kelas;
}

export interface Sesi {
	id: number;
	judul_sesi: string;
	deskripsi: string;
	urutan: number;
	is_ujian_praktik: boolean;
	courses?: Course[];
}

export interface Course {
	id: number;
	sesi_praktikum_id: number;
	jenis: string;
	judul: string;
	deskripsi: string;
	durasi_menit: number;
	kuota_easy?: number | null;
	kuota_medium?: number | null;
	kuota_hard?: number | null;
}

export interface Soal {
	id: number;
	course_id: number;
	jenis_soal: string;
	difficulty?: string | null;
	kategori_ujian?: string | null;
	teks_soal: string;
	gambar_url?: string | null;
	poin: number;
	kunci_jawaban?: string | null;
}

export interface CourseUserItem {
	course_id: number;
	aktivasi_course_id: number;
	jenis: string;
	judul: string;
	durasi_menit: number;
	is_open: boolean;
	status: string;
	total_nilai?: number | null;
}

export interface SesiUserItem {
	sesi_id: number;
	judul: string;
	deskripsi: string;
	urutan: number;
	is_ujian_praktik: boolean;
	aktif: boolean;
	susulan: boolean;
	aktivasi_sesi_id?: number | null;
	courses: CourseUserItem[];
}

export interface SoalTampil {
	soal_terpilih_id: number;
	urutan: number;
	jenis_soal: string;
	kategori_ujian?: string | null;
	teks_soal: string;
	gambar_url?: string | null;
	poin: number;
	jawaban_teks: string;
	is_submitted: boolean;
}

export interface RuangCourse {
	aktivasi_sesi_id: number;
	course_id: number;
	jenis: string;
	durasi_menit: number;
	waktu_mulai?: string | null;
	deadline?: string | null;
	status: string;
	is_open: boolean;
	require_token: boolean;
	soal: SoalTampil[];
}
