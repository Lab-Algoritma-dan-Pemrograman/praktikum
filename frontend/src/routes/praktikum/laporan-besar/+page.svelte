<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import Countdown from '$lib/components/Countdown.svelte';
	import {
		FileText,
		FileType2,
		Upload,
		CheckCircle2,
		AlertTriangle,
		Clock,
		Lock,
		Info,
		Send,
		Loader2,
		X
	} from 'lucide-svelte';

	// ---- Tipe sesuai kontrak backend 'Laporan Besar' ----
	interface LaporanBesar {
		id: number;
		judul: string;
		deskripsi: string;
		butuh_laporan: boolean;
		butuh_analisa: boolean;
		saya_berlaku: boolean;
		window: { buka_at: string | null; deadline_at: string | null; late_until: string | null };
		status_saya: 'belum' | 'terkumpul';
		sumber: string;
		is_late: boolean;
		submitted_at: string | null;
		laporan_url: string | null;
		analisa_url: string | null;
		dikunci: boolean;
	}

	interface UploadUrlRes {
		upload_url: string;
		path: string;
	}

	type Jenis = 'laporan' | 'analisa';
	type Slot = 'idle' | 'uploading' | 'done' | 'error';

	const MIME: Record<Jenis, string> = {
		laporan: 'application/pdf',
		analisa: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
	};
	const ACCEPT: Record<Jenis, string> = {
		laporan: '.pdf,application/pdf',
		analisa:
			'.docx,application/vnd.openxmlformats-officedocument.wordprocessingml.document'
	};
	const LABEL: Record<Jenis, string> = {
		laporan: 'Laporan (PDF)',
		analisa: 'Analisa (DOCX)'
	};

	let data = $state<LaporanBesar | null>(null);
	let loading = $state(true);
	let err = $state('');

	// Status unggah per slot.
	let slot: Record<Jenis, Slot> = $state({ laporan: 'idle', analisa: 'idle' });
	// Path Supabase hasil upload-url, per jenis.
	let paths: Record<Jenis, string | null> = $state({ laporan: null, analisa: null });
	// Nama file terpilih untuk ditampilkan.
	let namaFile: Record<Jenis, string | null> = $state({ laporan: null, analisa: null });
	let uploadErr = $state('');

	let submitting = $state(false);
	let submitErr = $state('');
	let hapusLoading = $state(false);

	async function load() {
		try {
			data = await api.get<LaporanBesar>('/api/praktikum/laporan-besar');
			// Selaraskan status slot dengan file yang sudah ada di server.
			if (data.laporan_url) {
				slot.laporan = 'done';
				paths.laporan = '__tersimpan__';
			}
			if (data.analisa_url) {
				slot.analisa = 'done';
				paths.analisa = '__tersimpan__';
			}
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(load);

	// ---- Status window: Belum dibuka / Sedang dikumpul / TELAT / Ditutup ----
	type Tahap = 'belum_dibuka' | 'dikumpul' | 'telat' | 'ditutup';

	let tahap = $state<Tahap>('dikumpul');

	function hitungTahap(w: LaporanBesar['window']): Tahap {
		const now = Date.now();
		const buka = w.buka_at ? new Date(w.buka_at).getTime() : null;
		const deadline = w.deadline_at ? new Date(w.deadline_at).getTime() : null;
		const late = w.late_until ? new Date(w.late_until).getTime() : null;

		if (buka && now < buka) return 'belum_dibuka';
		if (deadline && now > deadline) {
			if (late && now <= late) return 'telat';
			return 'ditutup';
		}
		return 'dikumpul';
	}

	$effect(() => {
		if (data) tahap = hitungTahap(data.window);
	});

	// Hitung mundur mengarah ke deadline (atau late_until bila sedang telat).
	let countdownTarget = $derived.by(() => {
		if (!data) return null;
		if (tahap === 'telat') return data.window.late_until;
		if (tahap === 'belum_dibuka') return data.window.buka_at;
		if (tahap === 'dikumpul') return data.window.deadline_at;
		return null;
	});

	let tahapLabel = $derived(
		tahap === 'belum_dibuka'
			? 'Belum dibuka'
			: tahap === 'telat'
				? 'TELAT (masih bisa)'
				: tahap === 'ditutup'
					? 'Ditutup'
					: 'Sedang dikumpul'
	);

	let tahapBadgeClass = $derived(
		tahap === 'belum_dibuka'
			? 'bg-slate-100 text-slate-500 border-slate-200'
			: tahap === 'dikumpul'
				? 'bg-state-success-bg text-state-success border-emerald-200'
				: tahap === 'telat'
					? 'bg-state-warning-bg text-state-warning border-amber-200'
					: 'bg-state-error-bg text-state-error border-red-200'
	);

	// ---- Format tanggal (locale Indonesia) ----
	const fmt = new Intl.DateTimeFormat('id-ID', {
		weekday: 'long',
		day: 'numeric',
		month: 'long',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
	function fmtTanggal(iso: string | null | undefined): string {
		if (!iso) return '—';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '—';
		return fmt.format(d) + ' WITA';
	}

	// Penalti keterlambatan (menampilkan 1/7..7/7). Hari penuh = ceil(selisih hari),
	// dibatasi 1..7. Murni informasi; tidak memengaruhi status.
	function formatPenalti(submitted: string, deadline: string): string {
		const s = new Date(submitted).getTime();
		const d = new Date(deadline).getTime();
		if (Number.isNaN(s) || Number.isNaN(d)) return '?/7';
		const telatHari = Math.max(1, Math.min(7, Math.ceil((s - d) / (24 * 60 * 60 * 1000))));
		return `${telatHari}/7`;
	}

	// ---- Apakah UI unggah boleh dipakai? ----
	let terkumpul = $derived(data?.status_saya === 'terkumpul');
	let manual = $derived(data?.sumber === 'manual');
	let terkunci = $derived(tahap === 'belum_dibuka' || tahap === 'ditutup' || !!data?.dikunci);
	// Bila sudah terkumpul: boleh unggah ulang HANYA bila belum ditutup,
	// belum diverifikasi manual, dan belum dikunci.
	let bolehUpload = $derived(
		tahap === 'dikumpul' || tahap === 'telat'
			? !terkumpul || (!manual && !terkunci)
			: false
	);

	function butuh(j: Jenis): boolean {
		if (!data) return false;
		return j === 'laporan' ? data.butuh_laporan : data.butuh_analisa;
	}

	// Semua file wajib sudah terunggah?
	let semuaSiap = $derived(
		(!data || !data.butuh_laporan || slot.laporan === 'done') &&
			(!data || !data.butuh_analisa || slot.analisa === 'done')
	);

	let adaProses = $derived(slot.laporan === 'uploading' || slot.analisa === 'uploading');

	async function pilihFile(j: Jenis, ev: Event) {
		uploadErr = '';
		const input = ev.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		// Validasi ekstensi sederhana di klien (server tetap memvalidasi).
		const extHarap = j === 'laporan' ? 'pdf' : 'docx';
		const extFile = (file.name.split('.').pop() || '').toLowerCase();
		if (extFile !== extHarap) {
			uploadErr = `File ${LABEL[j]} harus berekstensi .${extHarap}.`;
			input.value = '';
			return;
		}

		// Batas ukuran unggah: 10 MB (konsisten dengan bucket Supabase).
		const MAX_UPLOAD = 10 * 1024 * 1024;
		if (file.size > MAX_UPLOAD) {
			uploadErr = `File ${LABEL[j]} terlalu besar (${(file.size / 1024 / 1024).toFixed(1)} MB). Maksimal 10 MB.`;
			input.value = '';
			return;
		}

		namaFile[j] = file.name;
		slot[j] = 'uploading';
		try {
			// 1) Minta signed upload URL dari backend.
			const res = await api.post<UploadUrlRes>('/api/praktikum/laporan-besar/upload-url', {
				jenis: j,
				ext: extHarap
			});

			// 2) PUT langsung ke Supabase (BUKAN lewat api wrapper — URL eksternal).
			const put = await fetch(res.upload_url, {
				method: 'PUT',
				body: file,
				headers: { 'Content-Type': MIME[j] }
			});
			if (!put.ok) {
				throw new Error(`Unggah gagal ke penyimpanan (HTTP ${put.status}).`);
			}

			// 3) Simpan path untuk finalisasi.
			paths[j] = res.path;
			slot[j] = 'done';
		} catch (e) {
			slot[j] = 'error';
			uploadErr = (e as Error).message;
		} finally {
			input.value = '';
		}
	}

	async function kumpulkan() {
		if (!data) return;
		submitErr = '';
		// Bila file sudah tersimpan di server tapi belum diunggah ulang, jangan kirim
		// path placeholder ke backend.
		const laporan_path = data.butuh_laporan ? pathFinal('laporan') : null;
		const analisa_path = data.butuh_analisa ? pathFinal('analisa') : null;

		if (data.butuh_laporan && !laporan_path) {
			submitErr = 'File Laporan belum terunggah.';
			return;
		}
		if (data.butuh_analisa && !analisa_path) {
			submitErr = 'File Analisa belum terunggah.';
			return;
		}

		submitting = true;
		try {
			await api.post('/api/praktikum/laporan-besar/kumpul', { laporan_path, analisa_path });
			await load();
		} catch (e) {
			submitErr = (e as Error).message;
		} finally {
			submitting = false;
		}
	}

	// placeholder '__tersimpan__' berarti file sudah ada di server; kirim apa adanya
	// hanya bila user belum menggantinya -- untuk kasus unggah ulang, path nyata dipakai.
	function pathFinal(j: Jenis): string | null {
		const p = paths[j];
		if (!p || p === '__tersimpan__') return null;
		return p;
	}

	async function batal() {
		hapusLoading = true;
		submitErr = '';
		try {
			await api.del('/api/praktikum/laporan-besar/kumpul');
			slot.laporan = 'idle';
			slot.analisa = 'idle';
			paths.laporan = null;
			paths.analisa = null;
			namaFile.laporan = null;
			namaFile.analisa = null;
			await load();
		} catch (e) {
			submitErr = (e as Error).message;
		} finally {
			hapusLoading = false;
		}
	}
</script>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-center gap-3">
		<div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
			<FileText size={20} />
		</div>
		<div>
			<h1 class="text-2xl font-bold text-ink-heading">Tugas Akhir Praktikum</h1>
			<p class="text-sm text-ink-caption">Unggah laporan akhir dan analisa praktikum.</p>
		</div>
	</div>

	{#if loading}
		<p class="text-ink-caption">Memuat…</p>
	{:else if err}
		<p class="rounded-lg bg-state-error-bg p-3 text-state-error">{err}</p>
	{:else if data}
		<!-- Kartu info utama -->
		<section class="card fade-up">
			<div class="flex flex-wrap items-start justify-between gap-4">
				<div class="min-w-0">
					<div class="flex items-center gap-3">
						<h2 class="text-lg font-bold text-ink-heading">{data.judul}</h2>
						<span class="badge border {tahapBadgeClass}">{tahapLabel}</span>
						{#if data.status_saya === 'terkumpul'}
							<span class="badge border bg-state-success-bg text-state-success border-emerald-200">
								{data.is_late ? 'Terkumpul (telat)' : manual ? 'Terkumpul (manual — via MS Teams)' : 'Terkumpul'}
							</span>
						{/if}
					</div>
					<div class="prose prose-sm prose-slate max-w-2xl mt-2 break-words text-ink-body">
						{@html data.deskripsi || '<span class="text-ink-caption">(tidak ada deskripsi)</span>'}
					</div>
					{#if data.status_saya === 'terkumpul' && data.is_late && data.window.deadline_at}
						<div class="mt-3 inline-flex items-center gap-2 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 ring-1 ring-amber-200">
							<Clock size={14} />
							<span>
								Kamu mengumpulkan <strong>telat</strong> — keterlambatan
								{formatPenalti(data.submitted_at!, data.window.deadline_at)}.
								Hari penuh dihitung dari batas pengumpulan.
							</span>
						</div>
					{/if}
				</div>

				{#if countdownTarget && (tahap === 'dikumpul' || tahap === 'telat' || tahap === 'belum_dibuka')}
					<div class="flex flex-col items-end gap-1">
						<span class="text-xs text-ink-caption">
							{tahap === 'belum_dibuka'
								? 'Dibuka dalam'
								: tahap === 'telat'
									? 'Batas akhir (masih bisa)'
									: 'Sisa waktu'}
						</span>
						<Countdown deadline={countdownTarget} />
					</div>
				{/if}
			</div>

			<!-- Detail window -->
			<div class="mt-5 grid gap-3 sm:grid-cols-3">
				<div class="rounded-xl border border-slate-100 bg-slate-50/50 px-4 py-3 text-sm">
					<p class="text-xs text-ink-caption">Dibuka</p>
					<p class="font-semibold text-ink-heading">{fmtTanggal(data.window.buka_at)}</p>
				</div>
				<div class="rounded-xl border border-slate-100 bg-slate-50/50 px-4 py-3 text-sm">
					<p class="text-xs text-ink-caption">Batas Pengumpulan</p>
					<p class="font-semibold text-ink-heading">{fmtTanggal(data.window.deadline_at)}</p>
				</div>
				<div class="rounded-xl border border-slate-100 bg-slate-50/50 px-4 py-3 text-sm">
					<p class="text-xs text-ink-caption">Batas Toleransi Keterlambatan</p>
					<p class="font-semibold text-ink-heading">
						{data.window.late_until ? fmtTanggal(data.window.late_until) : 'Tidak ada'}
					</p>
				</div>
			</div>
		</section>

		<!-- Tidak berlaku untuk mahasiswa ini -->
		{#if !data.saya_berlaku}
			<div class="card flex items-start gap-3 border-l-4 border-l-blue-400">
				<span class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-state-info-bg text-state-info">
					<Info size={18} />
				</span>
				<div>
					<h3 class="font-semibold text-ink-heading">Tidak berlaku untuk kelas kamu</h3>
					<p class="mt-1 text-sm leading-relaxed text-ink-body">
						Laporan Besar ini tidak diberlakukan untuk kelas kamu. Kamu tidak perlu
						mengunggah dokumen apa pun di sini. Hubungi asisten praktikum bila kamu
						merasa hal ini keliru.
					</p>
				</div>
			</div>
		{:else}
			<!-- Jawaban manual (TE / MS Teams) -->
			{#if manual}
				<div class="card flex items-start gap-3 border-l-4 border-l-blue-400">
					<span class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-state-info-bg text-state-info">
						<Info size={18} />
					</span>
					<div>
						<h3 class="font-semibold text-ink-heading">Pengumpulan via MS Teams</h3>
						<p class="mt-1 text-sm leading-relaxed text-ink-body">
							Laporan Besar untuk kelasmu dikumpulkan melalui MS Teams. Status
							pengumpulanmu ditetapkan oleh asisten, bukan lewat halaman ini.
						</p>
					</div>
				</div>
			{/if}

			<!-- Terkunci / terverifikasi manual / ditutup -->
			{#if !bolehUpload && !terkumpul}
				<div class="card flex items-start gap-3 border-l-4 border-l-slate-300">
					<span class="mt-0.5 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-slate-100 text-slate-500">
						<Lock size={18} />
					</span>
					<div>
						<h3 class="font-semibold text-ink-heading">
							{tahap === 'belum_dibuka' ? 'Belum dibuka' : 'Pengumpulan ditutup'}
						</h3>
						<p class="mt-1 text-sm leading-relaxed text-ink-body">
							{tahap === 'belum_dibuka'
								? 'Pengumpulan Laporan Besar belum dibuka. Halaman ini akan mengizinkan unggah setelah periode dimulai.'
								: 'Periode pengumpulan Laporan Besar telah ditutup. Kamu tidak dapat mengunggah lagi.'}
						</p>
					</div>
				</div>
			{/if}

			<!-- Slot unggah -->
			{#if bolehUpload}
				<section class="space-y-4">
					<h2 class="text-lg font-bold text-ink-heading">Unggah Dokumen</h2>
					<p class="text-xs text-ink-caption">
						Format: laporan <strong>.pdf</strong>, analisa <strong>.docx</strong>. Maksimal
						<strong>10 MB</strong> per file.
					</p>

					{#if uploadErr}
						<div class="flex items-start gap-2 rounded-lg bg-state-error-bg p-3 text-sm text-state-error">
							<AlertTriangle size={16} class="mt-0.5 flex-shrink-0" />
							<span>{uploadErr}</span>
						</div>
					{/if}

					<div class="grid gap-4 md:grid-cols-2">
						{#each ['laporan', 'analisa'] as const as j}
							{#if butuh(j)}
								<div
									class="card flex flex-col {slot[j] === 'done'
										? 'border-emerald-200'
										: slot[j] === 'error'
											? 'border-red-200'
											: ''}"
								>
									<div class="flex items-start justify-between gap-3">
										<div class="flex items-center gap-3">
											<span
												class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-xl {slot[j] ===
												'done'
													? 'bg-state-success-bg text-state-success'
													: 'bg-primary/10 text-primary'}"
											>
												{#if j === 'laporan'}
													<FileText size={20} />
												{:else}
													<FileType2 size={20} />
												{/if}
											</span>
											<div>
												<h3 class="text-base font-bold text-ink-heading">{LABEL[j]}</h3>
												{#if namaFile[j]}
													<p class="truncate text-xs text-ink-caption">{namaFile[j]}</p>
												{:else}
													<p class="text-xs text-ink-caption">
														Format .{j === 'laporan' ? 'pdf' : 'docx'}
													</p>
												{/if}
											</div>
										</div>
									</div>

									<div class="mt-4 flex items-center justify-between gap-3">
										<div class="flex items-center gap-2 text-sm">
											{#if slot[j] === 'uploading'}
												<span class="inline-flex items-center gap-1.5 text-ink-caption">
													<Loader2 size={15} class="animate-spin" /> Mengunggah…
												</span>
											{:else if slot[j] === 'done'}
												<span class="inline-flex items-center gap-1.5 text-state-success">
													<CheckCircle2 size={15} /> Sudah terunggah
												</span>
											{:else if slot[j] === 'error'}
												<span class="inline-flex items-center gap-1.5 text-state-error">
													<AlertTriangle size={15} /> Gagal
												</span>
											{:else}
												<span class="text-ink-caption">Belum diunggah</span>
											{/if}
										</div>

										<label
											class="btn-outline cursor-pointer text-xs {slot[j] === 'uploading'
												? 'pointer-events-none opacity-50'
												: ''}"
										>
											<Upload size={14} />
											{slot[j] === 'done' ? 'Ganti' : 'Pilih'}
											<input
												type="file"
												class="hidden"
												accept={ACCEPT[j]}
												onchange={(e) => pilihFile(j, e)}
												disabled={slot[j] === 'uploading'}
											/>
										</label>
									</div>
								</div>
							{/if}
						{/each}
					</div>
				</section>

				<!-- Aksi finalisasi -->
				<section class="card">
					{#if submitErr}
						<div class="mb-4 flex items-start gap-2 rounded-lg bg-state-error-bg p-3 text-sm text-state-error">
							<AlertTriangle size={16} class="mt-0.5 flex-shrink-0" />
							<span>{submitErr}</span>
						</div>
					{/if}

					<div class="flex flex-wrap items-center justify-between gap-3">
						<div class="text-sm">
							{#if terkumpul}
								<p class="inline-flex items-center gap-1.5 text-state-success">
									<CheckCircle2 size={16} /> Laporan sudah terkumpul
								</p>
								<p class="text-xs text-ink-caption">
									Dikumpulkan: {fmtTanggal(data.submitted_at)}
								</p>
							{:else if semuaSiap}
								<p class="text-ink-body">Semua file wajib sudah terunggah. Siap dikumpulkan.</p>
							{:else}
								<p class="text-ink-caption">Unggah semua file wajib terlebih dahulu.</p>
							{/if}
						</div>

						<div class="flex flex-wrap items-center gap-2">
							{#if terkumpul && !manual}
								<button
									class="btn-danger"
									disabled={hapusLoading}
									onclick={batal}
								>
									{#if hapusLoading}
										<Loader2 size={15} class="animate-spin" />
									{:else}
										<X size={15} />
									{/if}
									Batalkan Pengumpulan
								</button>
							{:else}
								<button
									class="btn-primary"
									disabled={!semuaSiap || adaProses || terkumpul || submitting}
									onclick={kumpulkan}
								>
									{#if submitting}
										<Loader2 size={15} class="animate-spin" />
									{:else}
										<Send size={15} />
									{/if}
									{terkumpul ? 'Sudah Terkumpul' : 'Kumpulkan'}
								</button>
							{/if}
						</div>
					</div>

					{#if data.status_saya === 'terkumpul'}
						<div class="mt-4 border-t border-slate-100 pt-4">
							<p class="text-xs text-ink-caption">
								<span class="inline-flex items-center gap-1.5">
									<Clock size={13} /> Dokumen tersimpan aman dan hanya dapat dilihat olehmu dan asisten.
								</span>
							</p>
						</div>
					{/if}
				</section>
			{/if}
		{/if}
	{/if}
</div>
