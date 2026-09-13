<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { 
		Search, RefreshCw, ChevronLeft, ChevronRight, 
		Terminal, Calendar, Shield, User, Info, Globe, AlertCircle 
	} from 'lucide-svelte';
	import { fade } from 'svelte/transition';

	// Audit Log types
	interface AuditLog {
		id: number;
		user_id: number | null;
		nim: string;
		nama: string;
		role: string;
		action: string;
		description: string;
		ip_address: string;
		user_agent: string;
		created_at: string;
	}

	// Filter & pagination states
	let search = $state('');
	let role = $state('');
	let action = $state('');
	let page = $state(1);
	let limit = $state(20);

	let logs = $state<AuditLog[]>([]);
	let total = $state(0);
	let loading = $state(false);
	let err = $state('');

	// List of actions for dropdown filter
	const actionTypes = [
		{ value: 'LOGIN', label: 'Login Sukses' },
		{ value: 'LOGIN_FAILED', label: 'Login Gagal' },
		{ value: 'LOGOUT', label: 'Logout' },
		{ value: 'REGISTER', label: 'Registrasi Akun' },
		{ value: 'CREATE_USER', label: 'Tambah User' },
		{ value: 'UPDATE_USER', label: 'Perbarui User' },
		{ value: 'DELETE_USER', label: 'Hapus User' },
		{ value: 'RESET_PASSWORD', label: 'Reset Password' },
		{ value: 'CREATE_SOAL', label: 'Tambah Soal' },
		{ value: 'UPDATE_SOAL', label: 'Perbarui Soal' },
		{ value: 'DELETE_SOAL', label: 'Hapus Soal' },
		{ value: 'CREATE_SESI', label: 'Tambah Sesi' },
		{ value: 'UPDATE_SESI', label: 'Perbarui Sesi' },
		{ value: 'DELETE_SESI', label: 'Hapus Sesi' },
		{ value: 'CREATE_COURSE', label: 'Tambah Course' },
		{ value: 'UPDATE_COURSE', label: 'Perbarui Course' },
		{ value: 'DELETE_COURSE', label: 'Hapus Course' },
		{ value: 'SET_NILAI', label: 'Penilaian Jawaban' },
		{ value: 'SET_KEAKTIFAN', label: 'Penilaian Keaktifan' },
		{ value: 'SET_KONFIGURASI', label: 'Ubah Konfigurasi' }
	];

	async function loadLogs() {
		loading = true;
		err = '';
		try {
			const query = new URLSearchParams({
				search: search.trim(),
				role,
				action,
				page: page.toString(),
				limit: limit.toString()
			});
			const res = await api.get<{ logs: AuditLog[]; total: number; page: number; limit: number }>(
				`/api/admin/audit-logs?${query.toString()}`
			);
			if (res) {
				logs = res.logs ?? [];
				total = res.total ?? 0;
			}
		} catch (e) {
			err = (e as Error).message;
		} finally {
			loading = false;
		}
	}

	// Trigger load on state change
	$effect(() => {
		// Read state dependencies to trigger automatically
		const _search = search;
		const _role = role;
		const _action = action;
		const _page = page;
		const _limit = limit;
		
		// Debounce or load
		loadLogs();
	});

	function handleSearchInput() {
		page = 1; // reset page to 1 on filter change
	}

	function handleFilterChange() {
		page = 1; // reset page to 1 on filter change
	}

	// Utility to format date time
	function formatTime(dateStr: string) {
		if (!dateStr) return '';
		const d = new Date(dateStr);
		return d.toLocaleString('id-ID', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			hour12: false
		});
	}

	// Utility to parse User Agent simply
	function parseUA(ua: string) {
		if (!ua) return 'Tidak diketahui';
		let os = 'Unknown OS';
		if (ua.includes('Windows')) os = 'Windows';
		else if (ua.includes('Macintosh') || ua.includes('Mac OS')) os = 'macOS';
		else if (ua.includes('Linux')) os = 'Linux';
		else if (ua.includes('Android')) os = 'Android';
		else if (ua.includes('iPhone') || ua.includes('iPad')) os = 'iOS';

		let browser = 'Browser';
		if (ua.includes('Firefox')) browser = 'Firefox';
		else if (ua.includes('Chrome')) browser = 'Chrome';
		else if (ua.includes('Safari') && !ua.includes('Chrome')) browser = 'Safari';
		else if (ua.includes('Edge')) browser = 'Edge';

		return `${browser} (${os})`;
	}

	// Helper to color action tags
	function getActionClass(act: string) {
		if (act.includes('DELETE') || act.includes('FAILED')) {
			return 'bg-red-50 text-red-700 border border-red-200/50';
		}
		if (act.includes('CREATE') || act.includes('REGISTER') || act.includes('LOGIN')) {
			return 'bg-emerald-50 text-emerald-700 border border-emerald-200/50';
		}
		if (act.includes('UPDATE') || act.includes('SET')) {
			return 'bg-blue-50 text-blue-700 border border-blue-200/50';
		}
		return 'bg-slate-50 text-slate-700 border border-slate-200/50';
	}

	let totalPages = $derived(Math.ceil(total / limit));
</script>

<div class="space-y-6">
	<!-- HEADER BANNER -->
	<div class="flex flex-col md:flex-row md:items-center justify-between gap-4 bg-white p-6 md:p-8 rounded-[2rem] border border-slate-100 shadow-sm">
		<div class="flex items-center gap-4">
			<div class="w-12 h-12 rounded-2xl bg-gradient-to-br from-primary to-primary-hover text-rose-100 flex items-center justify-center shadow-md shadow-rose-100">
				<Terminal size={24} />
			</div>
			<div>
				<h1 class="text-2xl font-black text-slate-900 leading-tight">Log Aktivitas Sistem</h1>
				<p class="text-xs font-semibold text-slate-500 mt-1">Audit trail lengkap pengerjaan praktikum, manajemen soal, dan konfigurasi asisten.</p>
			</div>
		</div>
		<button 
			onclick={loadLogs} 
			disabled={loading}
			class="px-5 py-2.5 bg-slate-50 hover:bg-slate-100 text-slate-700 disabled:opacity-50 text-xs font-bold rounded-xl border border-slate-200/80 hover:border-slate-300 transition-all flex items-center gap-2 self-start md:self-auto"
		>
			<RefreshCw size={14} class={loading ? 'animate-spin' : ''} />
			Muat Ulang
		</button>
	</div>

	<!-- FILTERS PANEL -->
	<div class="bg-white p-5 rounded-[2rem] border border-slate-100 shadow-sm space-y-4">
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
			<!-- Search query -->
			<div class="relative">
				<Search class="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" size={16} />
				<input
					type="text"
					placeholder="Cari Nama / NIM / Deskripsi..."
					bind:value={search}
					oninput={handleSearchInput}
					class="w-full pl-10 pr-4 py-2.5 rounded-xl border border-slate-200 text-xs text-slate-800 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all"
				/>
			</div>

			<!-- Role select -->
			<div class="relative">
				<select
					bind:value={role}
					onchange={handleFilterChange}
					class="w-full px-4 py-2.5 rounded-xl border border-slate-200 text-xs text-slate-800 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all bg-white appearance-none cursor-pointer"
				>
					<option value="">Semua Peran (Roles)</option>
					<option value="koordinator">Koordinator</option>
					<option value="asisten">Asisten</option>
					<option value="mahasiswa">Mahasiswa</option>
					<option value="guest">Tamu (guest)</option>
					<option value="system">Sistem (system)</option>
				</select>
				<div class="absolute right-3.5 top-1/2 -translate-y-1/2 pointer-events-none border-l-4 border-r-4 border-t-4 border-transparent border-t-slate-500"></div>
			</div>

			<!-- Action type select -->
			<div class="relative">
				<select
					bind:value={action}
					onchange={handleFilterChange}
					class="w-full px-4 py-2.5 rounded-xl border border-slate-200 text-xs text-slate-800 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all bg-white appearance-none cursor-pointer"
				>
					<option value="">Semua Jenis Aktivitas</option>
					{#each actionTypes as act}
						<option value={act.value}>{act.label}</option>
					{/each}
				</select>
				<div class="absolute right-3.5 top-1/2 -translate-y-1/2 pointer-events-none border-l-4 border-r-4 border-t-4 border-transparent border-t-slate-500"></div>
			</div>

			<!-- Page limit -->
			<div class="relative">
				<select
					bind:value={limit}
					onchange={handleFilterChange}
					class="w-full px-4 py-2.5 rounded-xl border border-slate-200 text-xs text-slate-800 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all bg-white appearance-none cursor-pointer"
				>
					<option value={10}>10 Baris per Halaman</option>
					<option value={20}>20 Baris per Halaman</option>
					<option value={50}>50 Baris per Halaman</option>
					<option value={100}>100 Baris per Halaman</option>
				</select>
				<div class="absolute right-3.5 top-1/2 -translate-y-1/2 pointer-events-none border-l-4 border-r-4 border-t-4 border-transparent border-t-slate-500"></div>
			</div>
		</div>
	</div>

	<!-- DATA TABLE CARD -->
	<div class="bg-white rounded-[2rem] border border-slate-100 shadow-sm overflow-hidden relative">
		
		{#if loading}
			<div class="absolute inset-0 bg-white/70 backdrop-blur-[1px] z-20 flex items-center justify-center" transition:fade={{ duration: 150 }}>
				<div class="flex flex-col items-center gap-3">
					<RefreshCw size={28} class="text-primary animate-spin" />
					<span class="text-xs font-bold text-slate-600">Memuat log aktivitas...</span>
				</div>
			</div>
		{/if}

		<div class="w-full overflow-x-auto scrollbar-thin scrollbar-thumb-primary/20">
			<table class="w-full text-left border-collapse min-w-[900px]">
				<thead>
					<tr class="bg-slate-50/50 border-b border-slate-100">
						<th class="px-6 py-4 text-xs font-bold text-slate-500 tracking-wider w-[180px]">
							<div class="flex items-center gap-1.5"><Calendar size={13} /> Waktu &amp; Tanggal</div>
						</th>
						<th class="px-6 py-4 text-xs font-bold text-slate-500 tracking-wider w-[220px]">
							<div class="flex items-center gap-1.5"><User size={13} /> Pelaku</div>
						</th>
						<th class="px-6 py-4 text-xs font-bold text-slate-500 tracking-wider w-[120px]">
							<div class="flex items-center gap-1.5"><Shield size={13} /> Peran</div>
						</th>
						<th class="px-6 py-4 text-xs font-bold text-slate-500 tracking-wider w-[160px]">
							<div class="flex items-center gap-1.5"><Terminal size={13} /> Aksi</div>
						</th>
						<th class="px-6 py-4 text-xs font-bold text-slate-500 tracking-wider">
							<div class="flex items-center gap-1.5"><Info size={13} /> Deskripsi Kegiatan</div>
						</th>
						<th class="px-6 py-4 text-xs font-bold text-slate-500 tracking-wider w-[180px]">
							<div class="flex items-center gap-1.5"><Globe size={13} /> Klien / IP</div>
						</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-slate-100">
					{#each logs as log (log.id)}
						<tr class="hover:bg-slate-50/30 transition-colors">
							<!-- Time -->
							<td class="px-6 py-3.5 text-[11px] font-mono text-slate-600">
								{formatTime(log.created_at)}
							</td>
							<!-- User details -->
							<td class="px-6 py-3.5">
								<div class="flex flex-col">
									<span class="text-xs font-bold text-slate-900">{log.nama || 'System'}</span>
									{#if log.nim}
										<span class="text-[10px] font-mono font-semibold text-slate-400 mt-0.5">{log.nim}</span>
									{/if}
								</div>
							</td>
							<!-- Role badge -->
							<td class="px-6 py-3.5">
								{#if log.role === 'koordinator'}
								<span class="px-2 py-0.5 rounded-md text-[9px] font-extrabold uppercase bg-violet-50 text-violet-700 border border-violet-200/50">
									Superadmin
								</span>
							{:else if log.role === 'asisten'}
									<span class="px-2 py-0.5 rounded-md text-[9px] font-extrabold uppercase bg-primary/10 text-[#8A1538] border border-rose-100/50">
										Asisten
									</span>
								{:else if log.role === 'mahasiswa'}
									<span class="px-2 py-0.5 rounded-md text-[9px] font-extrabold uppercase bg-slate-100 text-slate-600 border border-slate-200/50">
										Praktikan
									</span>
								{:else if log.role === 'guest'}
									<span class="px-2 py-0.5 rounded-md text-[9px] font-extrabold uppercase bg-amber-50 text-amber-700 border border-amber-200/50">
										Tamu
									</span>
								{:else}
									<span class="px-2 py-0.5 rounded-md text-[9px] font-extrabold uppercase bg-indigo-50 text-indigo-700 border border-indigo-200/50">
										Sistem
									</span>
								{/if}
							</td>
							<!-- Action badge -->
							<td class="px-6 py-3.5">
								<span class="px-2 py-1 rounded-lg text-[10px] font-mono font-black uppercase {getActionClass(log.action)}">
									{log.action}
								</span>
							</td>
							<!-- Description -->
							<td class="px-6 py-3.5 text-xs text-slate-700 font-semibold leading-relaxed">
								{log.description}
							</td>
							<!-- IP & User agent details -->
							<td class="px-6 py-3.5">
								<div class="flex flex-col gap-0.5">
									<span class="text-[10px] font-mono font-bold text-slate-500">{log.ip_address}</span>
									<span class="text-[9px] font-medium text-slate-400 leading-tight truncate max-w-[160px]" title={log.user_agent}>
										{parseUA(log.user_agent)}
									</span>
								</div>
							</td>
						</tr>
					{:else}
						{#if !loading}
							<tr>
								<td colspan="6" class="px-6 py-12 text-center">
									<div class="flex flex-col items-center gap-2 max-w-sm mx-auto text-slate-400">
										<AlertCircle size={32} class="text-slate-300" />
										<span class="text-xs font-bold mt-1">Tidak ada data log ditemukan</span>
										<span class="text-[11px] font-medium text-slate-500">Coba ubah kata kunci pencarian atau bersihkan filter yang aktif.</span>
									</div>
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		</div>

		<!-- PAGINATION FOOTER -->
		{#if totalPages > 1}
			<div class="px-6 py-4 bg-slate-50/30 border-t border-slate-100 flex flex-col sm:flex-row items-center justify-between gap-4">
				<div class="text-xs font-semibold text-slate-500">
					Menampilkan <span class="font-bold text-slate-800">{logs.length}</span> dari <span class="font-bold text-slate-800">{total}</span> baris log
				</div>

				<div class="flex items-center gap-1">
					<button
						onclick={() => page = Math.max(1, page - 1)}
						disabled={page === 1}
						class="w-8 h-8 rounded-lg bg-white border border-slate-200 text-slate-600 flex items-center justify-center hover:bg-slate-50 hover:border-slate-300 transition-all disabled:opacity-40"
					>
						<ChevronLeft size={16} />
					</button>

					{#each Array(totalPages) as _, i}
						{@const p = i + 1}
						{#if p === 1 || p === totalPages || (p >= page - 1 && p <= page + 1)}
							<button
								onclick={() => page = p}
								class="w-8 h-8 rounded-lg text-xs font-bold border transition-all {page === p ? 'bg-primary text-white border-primary shadow-sm shadow-rose-100' : 'bg-white text-slate-600 border-slate-200 hover:bg-slate-50 hover:border-slate-300'}"
							>
								{p}
							</button>
						{:else if p === 2 || p === totalPages - 1}
							<span class="w-8 text-center text-xs font-bold text-slate-400">...</span>
						{/if}
					{/each}

					<button
						onclick={() => page = Math.min(totalPages, page + 1)}
						disabled={page === totalPages}
						class="w-8 h-8 rounded-lg bg-white border border-slate-200 text-slate-600 flex items-center justify-center hover:bg-slate-50 hover:border-slate-300 transition-all disabled:opacity-40"
					>
						<ChevronRight size={16} />
					</button>
				</div>
			</div>
		{/if}
	</div>
</div>
