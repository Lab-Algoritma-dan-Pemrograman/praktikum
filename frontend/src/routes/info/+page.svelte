<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import { 
		Calendar, Users, FileText, Book, ArrowRight, Bell, Code,
		Terminal, Clock, Download, ArrowDown, ExternalLink,
		MapPin, Mail, Phone, Compass, Cpu, Wifi, MonitorPlay, Building2,
		UserCheck, AlertTriangle, Copy, Image, X
	} from 'lucide-svelte';
	import { api } from '$lib/api';
	import type { User, Jadwal } from '$lib/types';

	// ─── Import Gallery Photos dynamically at build time ───
	const modules = import.meta.glob('$lib/galeri/*.{png,jpg,jpeg,webp,gif,PNG,JPG,JPEG,WEBP,GIF}', { eager: true, import: 'default' });
	
	interface PhotoItem {
		filename: string;
		src: string;
		title: string;
	}

	const photos = Object.keys(modules)
		.map(key => {
			const filename = key.split('/').pop() || '';
			// Naming convention: YYYYMMDD_[index]_title.ext (e.g. "20260705_1_foto-asisten.png")
			const match = filename.match(/^(\d{8})(?:_(\d+))?_(.*)\.[^/.]+$/);
			
			let dateStr = filename.slice(0, 8);
			let index = 999; // default high index for unindexed photos (placed last on the same day)
			let cleanName = filename.replace(/^\d+_/ , '').replace(/\.[^/.]+$/, '');

			if (match) {
				dateStr = match[1];
				if (match[2]) {
					index = parseInt(match[2], 10);
				}
				cleanName = match[3];
			}

			const title = cleanName
				.split(/[-_]/)
				.map(word => word.charAt(0).toUpperCase() + word.slice(1))
				.join(' ');

			return {
				filename,
				dateStr,
				index,
				src: modules[key] as string,
				title: title || 'Kegiatan Lab'
			};
		})
		.sort((a, b) => {
			// 1. Sort by date descending (newest first)
			if (a.dateStr !== b.dateStr) {
				return b.dateStr.localeCompare(a.dateStr);
			}
			// 2. If same date, sort by index ascending (lower index first, e.g. 1 shows before 2)
			if (a.index !== b.index) {
				return a.index - b.index;
			}
			// 3. Fallback to alphabetical sorting of filename
			return a.filename.localeCompare(b.filename);
		});

	let activePhotoIndex = $state(0);
	let autoplayInterval: any;
	let isHovered = false;

	function startAutoplay(delay = 5000) {
		if (autoplayInterval) {
			clearInterval(autoplayInterval);
		}
		if (photos.length > 1 && !isHovered) {
			autoplayInterval = setInterval(() => {
				nextPhoto();
			}, delay);
		}
	}

	function resetAutoplay() {
		// When user clicks manually, give them 15 seconds of peace before resuming 5s auto-sliding
		startAutoplay(15000);
	}

	function prevPhoto() {
		if (photos.length === 0) return;
		activePhotoIndex = (activePhotoIndex - 1 + photos.length) % photos.length;
		resetAutoplay();
	}

	function nextPhoto() {
		if (photos.length === 0) return;
		activePhotoIndex = (activePhotoIndex + 1) % photos.length;
		resetAutoplay();
	}

	function handleMouseEnter() {
		isHovered = true;
		if (autoplayInterval) {
			clearInterval(autoplayInterval);
		}
	}

	function handleMouseLeave() {
		isHovered = false;
		startAutoplay(5000); // Resume normal 5s autoplay
	}

	// ─── Announcements loaded from localStorage (managed via /info/pengaturan) ───
	interface RescheduleCard {
		id: string; kelas: string; hari: string; jam: string; catatan: string;
	}
	interface RecruitCard {
		id: string; judul: string; deskripsi: string; link?: string;
	}
	interface SusulanCard {
		id: string; kelas: string; deskripsi: string;
	}
	interface PlagiarismRow {
		nim: string; nama: string; keterangan: string; hari?: string; jam?: string;
	}
	interface PlagiarismData {
		rows: PlagiarismRow[]; updatedAt: string;
	}

	let reschedules = $state<RescheduleCard[]>([]);
	let recruitCards = $state<RecruitCard[]>([]);
	let susulanCards = $state<SusulanCard[]>([]);
	let plagiarismData = $state<PlagiarismData>({ rows: [], updatedAt: '' });
	let showPlagiarismModal = $state(false);

	// ─── Asisten for the team card (real data) ───
	let asisten = $state<User[]>([]);

	// ─── Active jadwal for the schedule bento card ───
	let jadwalList = $state<Jadwal[]>([]);

	// ─── Compute current active shift based on time ───
	let now = $state(new Date());

	let activeShift = $derived(() => {
		const day = ['Minggu','Senin','Selasa','Rabu','Kamis','Jumat','Sabtu'][now.getDay()];
		const hhmm = now.getHours() * 60 + now.getMinutes();
		return jadwalList.find(j => {
			if (j.hari !== day) return false;
			const [sh, sm] = j.jam_mulai.split(':').map(Number);
			const [eh, em] = j.jam_selesai.split(':').map(Number);
			return hhmm >= sh * 60 + sm && hhmm <= eh * 60 + em;
		}) ?? null;
	});

	onMount(() => {
		// Load announcements from backend database API
		api.get<{ reschedules?: string; recruit?: string; susulan?: string; plagiarism?: string }>(`/api/info/announcements?t=${Date.now()}`)
			.then(d => {
				if (d) {
					if (d.reschedules) reschedules = JSON.parse(d.reschedules);
					if (d.recruit) recruitCards = JSON.parse(d.recruit);
					if (d.susulan) susulanCards = JSON.parse(d.susulan);
					if (d.plagiarism) plagiarismData = JSON.parse(d.plagiarism);
				}
			})
			.catch(() => {
				// Fallback to localStorage if API is down
				try {
					const r = localStorage.getItem('ann_reschedules'); if (r) reschedules = JSON.parse(r);
					const rc = localStorage.getItem('ann_recruit');    if (rc) recruitCards = JSON.parse(rc);
					const s = localStorage.getItem('ann_susulan');     if (s) susulanCards = JSON.parse(s);
					const p = localStorage.getItem('ann_plagiarism');  if (p) plagiarismData = JSON.parse(p);
				} catch { /* ignore */ }
			});

		// Load asisten
		api.get<User[]>('/api/info/asisten').then(d => { asisten = d ?? []; }).catch(() => {});

		// Load jadwal for active shift detection
		api.get<Jadwal[]>('/api/info/jadwal').then(d => { jadwalList = d ?? []; }).catch(() => {});

		// Tick clock every minute
		const ticker = setInterval(() => { now = new Date(); }, 60_000);

		// Autoplay gallery if there are multiple photos
		startAutoplay(5000);

		// Scroll reveal
		const observer = new IntersectionObserver((entries) => {
			entries.forEach(e => {
				if (e.isIntersecting) e.target.classList.add('active');
				else e.target.classList.remove('active');
			});
		}, { threshold: 0.12 });
		document.querySelectorAll('.reveal').forEach(el => observer.observe(el));

		return () => {
			clearInterval(ticker);
			if (autoplayInterval) {
				clearInterval(autoplayInterval);
			}
			document.querySelectorAll('.reveal').forEach(el => observer.unobserve(el));
		};
	});

	// Show only first 3 asisten in preview
	let asistenPreview = $derived(asisten.slice(0, 4));
	let asistenExtra = $derived(Math.max(0, asisten.length - 4));

	// Shift display list (up to 4, mark active)
	let shiftJadwal = $derived(jadwalList.slice(0, 4).map(j => ({
		...j,
		active: activeShift()?.id === j.id
	})));
</script>

<!-- Background orbs + grid -->
<div class="fixed inset-0 overflow-hidden pointer-events-none z-0">
	<div class="absolute -top-[15%] -left-[10%] w-[45%] h-[45%] rounded-full bg-primary/10 blur-[120px]"></div>
	<div class="absolute top-[30%] -right-[10%] w-[35%] h-[55%] rounded-full bg-fun-blue/10 blur-[130px]"></div>
	<div class="absolute bottom-[5%] left-[20%] w-[30%] h-[30%] rounded-full bg-fun-yellow/8 blur-[100px]"></div>
	<div class="absolute top-[60%] right-[25%] w-[20%] h-[20%] rounded-full bg-fun-purple/8 blur-[90px]"></div>
	<div class="absolute inset-0 opacity-[0.12]"
		style="background-size: 40px 40px; background-image: linear-gradient(to right, rgba(138,21,56,0.12) 1px, transparent 1px), linear-gradient(to bottom, rgba(138,21,56,0.12) 1px, transparent 1px);">
	</div>
</div>

<div class="relative z-10 space-y-16">

	<!-- ─── HERO ─── -->
	<section class="max-w-3xl mx-auto text-center flex flex-col items-center pt-8 reveal">
		<h1 class="text-4xl md:text-5xl lg:text-6xl font-extrabold text-slate-900 leading-[1.15] tracking-tight mb-6">
			Laboratorium <br class="hidden sm:block">
			<span class="text-transparent bg-clip-text bg-gradient-to-r from-fun-blue via-fun-purple to-primary">Algoritma</span>
			&amp; <span class="text-transparent bg-clip-text bg-gradient-to-r from-primary to-primary/70">Pemrograman.</span>
		</h1>

		<!-- Dynamic aspect-ratio photo gallery of lab assistants (supports mixed landscape/portrait via blurred background technique) -->
		{#if photos.length > 0}
			<div class="w-full max-w-4xl mx-auto mt-6 mb-8 rounded-[2rem] overflow-hidden shadow-md border border-rose-100/40 aspect-[1.77/1] relative group" onmouseenter={handleMouseEnter} onmouseleave={handleMouseLeave}>
				{#each photos as photo, idx}
					{#if idx === activePhotoIndex}
						<div class="absolute inset-0" transition:fade={{ duration: 400 }}>
							<!-- Blurred background replica for portrait/unfitted images -->
							<img src={photo.src} alt="" class="absolute inset-0 w-full h-full object-cover blur-2xl scale-110 opacity-30 select-none pointer-events-none" />
							
							<!-- Main centered image (object-contain ensures the entire photo is visible without cropping) -->
							<img src={photo.src} alt={photo.title} class="absolute inset-0 w-full h-full object-contain z-10" />
							
							<!-- Soft overlay at the bottom for caption readability -->
							<div class="absolute inset-0 bg-gradient-to-t from-slate-950/75 via-slate-950/5 to-transparent z-20"></div>
							
							<!-- Caption overlay -->
							<div class="absolute bottom-6 left-6 right-6 text-left text-white drop-shadow-sm pointer-events-none z-30">
								<span class="bg-[#8A1538]/90 text-rose-100 text-[10px] font-black px-2.5 py-1 rounded-md border border-rose-800/40 uppercase tracking-widest">
									{photo.dateStr.slice(0, 4)}-{photo.dateStr.slice(4, 6)}-{photo.dateStr.slice(6, 8)}
								</span>
								<h3 class="text-sm md:text-xl font-black mt-1.5">{photo.title}</h3>
							</div>
						</div>
					{/if}
				{/each}

				<!-- Navigation Buttons (show only if there are multiple photos) -->
				{#if photos.length > 1}
					<button class="absolute left-4 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full bg-black/30 hover:bg-black/50 border border-white/10 text-white flex items-center justify-center transition-all opacity-0 group-hover:opacity-100 active:scale-95 z-40" onclick={prevPhoto} aria-label="Previous photo">
						<ArrowRight class="rotate-180" size={18} />
					</button>
					<button class="absolute right-4 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full bg-black/30 hover:bg-black/50 border border-white/10 text-white flex items-center justify-center transition-all opacity-0 group-hover:opacity-100 active:scale-95 z-40" onclick={nextPhoto} aria-label="Next photo">
						<ArrowRight size={18} />
					</button>

					<!-- Dots Indicator -->
					<div class="absolute bottom-4 right-6 flex gap-1.5 z-40">
						{#each photos as _, idx}
							<button class="w-1.5 h-1.5 rounded-full transition-all {idx === activePhotoIndex ? 'bg-white w-4' : 'bg-white/40'}" onclick={() => activePhotoIndex = idx} aria-label="Go to photo {idx + 1}"></button>
						{/each}
					</div>
				{/if}
			</div>
		{/if}

		<div class="flex flex-wrap justify-center gap-4">
			<a href="https://elearning.algohub.web.id" target="_blank" rel="noopener"
				class="px-8 py-3.5 rounded-xl text-sm font-bold text-white bg-gradient-to-r from-primary to-primary-hover shadow-md hover:shadow-lg hover:brightness-110 transition-all transform hover:-translate-y-0.5 flex items-center gap-2">
				<Terminal size={16} /> Buka Live Compiler
			</a>
			<a href="#pengumuman"
				class="px-8 py-3.5 rounded-xl text-sm font-bold text-slate-800 bg-white border border-slate-300 hover:border-fun-blue/50 hover:bg-fun-blue/5 shadow-sm transition-all flex items-center gap-2">
				Lihat Pengumuman <ArrowDown size={16} />
			</a>
		</div>
	</section>

	<!-- ─── PAPAN INFORMASI ─── -->
	<section id="pengumuman" class="scroll-mt-28">
		<div class="flex items-center justify-between mb-6 reveal">
			<div class="flex items-center gap-3">
				<div class="w-10 h-10 rounded-xl bg-gradient-to-br from-orange-400 to-amber-500 text-white flex items-center justify-center shadow-md shadow-orange-200">
					<Bell size={20} />
				</div>
				<h2 class="text-2xl font-extrabold text-slate-900 tracking-tight">Papan Informasi</h2>
			</div>
		</div>

		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 reveal delay-100">
			<!-- Reschedule cards -->
			{#if reschedules.length === 0}
				<div class="bg-white border border-amber-100 p-5 rounded-[1.5rem] flex gap-4 items-start relative overflow-hidden shadow-sm">
					<div class="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-amber-400 to-orange-400 rounded-l-[1.5rem]"></div>
					<div class="w-10 h-10 rounded-full bg-amber-50 text-amber-600 flex items-center justify-center shrink-0 border border-amber-200 ml-1">
						<Calendar size={18} />
					</div>
					<div class="text-left">
						<h4 class="font-extrabold text-slate-900 text-sm">Tidak Ada Reschedule</h4>
						<p class="text-xs text-slate-600 font-semibold leading-relaxed mt-1">Jadwal praktikum berjalan normal.</p>
					</div>
				</div>
			{:else}
				{#each reschedules as rs}
					<div class="bg-white border border-amber-100 p-5 rounded-[1.5rem] flex gap-4 items-start relative overflow-hidden group shadow-sm transition-all hover:border-amber-300 hover:shadow-md">
						<div class="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-amber-400 to-orange-400 rounded-l-[1.5rem]"></div>
						<div class="w-10 h-10 rounded-full bg-amber-50 text-amber-600 flex items-center justify-center shrink-0 border border-amber-200 ml-1">
							<Calendar size={18} />
						</div>
						<div class="text-left min-w-0">
							<div class="flex justify-between items-start mb-1 gap-2">
								<h4 class="font-extrabold text-slate-900 text-sm group-hover:text-amber-700 transition-colors">Reschedule {rs.kelas}</h4>
							</div>
							<p class="text-xs font-bold text-amber-700 mb-1">{rs.hari} · {rs.jam}</p>
							<p class="text-xs text-slate-700 font-semibold leading-relaxed">{rs.catatan}</p>
						</div>
					</div>
				{/each}
			{/if}

			<!-- Recruitment cards -->
			{#each recruitCards as rc}
				<div class="bg-white border border-blue-100 p-5 rounded-[1.5rem] flex gap-4 items-start relative overflow-hidden group shadow-sm transition-all hover:border-blue-300 hover:shadow-md">
					<div class="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-fun-blue to-blue-600 rounded-l-[1.5rem]"></div>
					<div class="w-10 h-10 rounded-full bg-blue-50 text-blue-600 flex items-center justify-center shrink-0 border border-blue-200 ml-1">
						<UserCheck size={18} />
					</div>
					<div class="text-left min-w-0 flex-1">
						<h4 class="font-extrabold text-slate-900 text-sm group-hover:text-blue-700 transition-colors">{rc.judul}</h4>
						<p class="text-xs text-slate-700 font-semibold leading-relaxed mt-1">{rc.deskripsi}</p>
						{#if rc.link}
							<a href={rc.link} target="_blank" rel="noopener" class="mt-2 inline-flex items-center gap-1 text-xs font-bold text-blue-600 hover:underline">
								Info Lengkap <ExternalLink size={11} />
							</a>
						{/if}
					</div>
				</div>
			{/each}

			<!-- Susulan cards -->
			{#each susulanCards as su}
				<div class="bg-white border border-purple-100 p-5 rounded-[1.5rem] flex gap-4 items-start relative overflow-hidden group shadow-sm transition-all hover:border-purple-300 hover:shadow-md">
					<div class="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-fun-purple to-indigo-600 rounded-l-[1.5rem]"></div>
					<div class="w-10 h-10 rounded-full bg-purple-50 text-purple-600 flex items-center justify-center shrink-0 border border-purple-200 ml-1">
						<Copy size={18} />
					</div>
					<div class="text-left min-w-0">
						<h4 class="font-extrabold text-slate-900 text-sm group-hover:text-purple-700 transition-colors">Susulan — {su.kelas}</h4>
						<p class="text-xs text-slate-700 font-semibold leading-relaxed mt-1">{su.deskripsi}</p>
					</div>
				</div>
			{/each}

			<!-- Plagiarism notice -->
			{#if plagiarismData.rows.length > 0}
				<div class="bg-white border border-red-100 p-5 rounded-[1.5rem] flex gap-4 items-start relative overflow-hidden group shadow-sm transition-all hover:border-red-300 hover:shadow-md col-span-1 md:col-span-2 lg:col-span-1">
					<div class="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-red-500 to-rose-600 rounded-l-[1.5rem]"></div>
					<div class="w-10 h-10 rounded-full bg-red-50 text-red-600 flex items-center justify-center shrink-0 border border-red-200 ml-1">
						<AlertTriangle size={18} />
					</div>
					<div class="text-left min-w-0 flex-1">
						<h4 class="font-extrabold text-slate-900 text-sm group-hover:text-red-700 transition-colors">Pelanggaran Plagiarisme</h4>
						<p class="text-xs text-slate-700 font-semibold leading-relaxed mt-1">{plagiarismData.rows.length} mahasiswa terdaftar. Data diperbarui {plagiarismData.updatedAt || '—'}.</p>
						<button onclick={() => showPlagiarismModal = true} class="mt-2 inline-flex items-center gap-1 text-xs font-bold text-red-600 hover:underline">
							Lihat Daftar <ArrowRight size={11} />
						</button>
					</div>
				</div>
			{/if}
		</div>
	</section>

	<!-- ─── BENTO GRID ─── -->
	<section class="space-y-6">

	<!-- ROW 1: Live Compiler & Terminal Stack (2 cols) + Jadwal (1 col) -->
	<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">

		<!-- Left main stack (col-span-2 containing both cards stacked vertically) -->
		<div class="lg:col-span-2 flex flex-col gap-6">

			<!-- 1. LIVE CODE EDITOR (full width of col-span-2) -->
			<a href="https://elearning.algohub.web.id" target="_blank" rel="noopener"
				class="h-full flex flex-col sm:flex-row items-center gap-6 group overflow-hidden relative rounded-[2rem] p-6 lg:p-8 shadow-lg reveal delay-100 hover:shadow-xl transition-all hover:-translate-y-0.5"
				style="background: linear-gradient(135deg, #1e293b 0%, #160f38 60%, #221454 100%); border: 1px solid #312e81;">
				<div class="absolute inset-0 bg-gradient-to-br from-fun-blue/5 via-fun-purple/10 to-transparent pointer-events-none z-0"></div>
				
				<!-- Left: text -->
				<div class="relative z-10 flex-1 text-left min-w-0 self-center">
					<span class="px-2.5 py-0.5 bg-fun-blue/20 rounded-full text-[9px] font-black tracking-wider uppercase border border-fun-blue/30 inline-flex items-center gap-1 mb-3 text-fun-blue">
						<Code size={11} /> Fitur Praktikum
					</span>
					<h3 class="text-2xl font-extrabold tracking-tight mb-2 text-white group-hover:text-fun-blue transition-colors">Live Code Editor</h3>
					<p class="text-slate-300 text-xs font-semibold leading-relaxed mb-4">
						Tulis kode C &amp; Python dengan editor Monaco premium yang dilengkapi fitur syntax highlighting lengkap langsung di browser.
					</p>
					<div class="inline-flex items-center gap-1 text-xs font-bold text-fun-blue group-hover:gap-2 transition-all">
						Buka Editor <ArrowRight size={12} />
					</div>
				</div>

				<!-- Right: editor mockup -->
				<div class="relative z-10 shrink-0 w-full sm:w-64 bg-slate-950 rounded-xl border border-slate-800 shadow-2xl overflow-hidden transform group-hover:-translate-y-1 transition-transform duration-500">
					<div class="bg-slate-900 px-3 py-1.5 flex items-center gap-1.5 border-b border-slate-800">
						<div class="flex gap-1">
							<div class="w-1.5 h-1.5 rounded-full bg-red-500"></div>
							<div class="w-1.5 h-1.5 rounded-full bg-amber-500"></div>
							<div class="w-1.5 h-1.5 rounded-full bg-green-500"></div>
						</div>
						<span class="text-[8px] font-mono text-slate-400 ml-1">main.c</span>
					</div>
					<div class="p-3 text-[9px] font-mono leading-relaxed text-left border-b border-slate-800/60">
						<div class="flex gap-2">
							<span class="text-slate-600 select-none w-3 text-right shrink-0">1</span>
							<span><span class="text-pink-400">#include</span> <span class="text-orange-300">&lt;stdio.h&gt;</span></span>
						</div>
						<div class="flex gap-2">
							<span class="text-slate-600 select-none w-3 text-right shrink-0">2</span>
							<span><span class="text-sky-400">int</span> <span class="text-fun-green">main</span><span class="text-yellow-300">() &#123;</span></span>
						</div>
						<div class="flex gap-2">
							<span class="text-slate-600 select-none w-3 text-right shrink-0">3</span>
							<span class="ml-3"><span class="text-fun-green">printf</span><span class="text-slate-300">(</span><span class="text-orange-300">"Halo Lab AP\n"</span><span class="text-slate-300">);</span></span>
						</div>
						<div class="flex gap-2">
							<span class="text-slate-600 select-none w-3 text-right shrink-0">4</span>
							<span class="ml-3"><span class="text-pink-400">return</span> <span class="text-fun-purple">0</span><span class="text-slate-300">;</span></span>
						</div>
						<div class="flex gap-2">
							<span class="text-slate-600 select-none w-3 text-right shrink-0">5</span>
							<span><span class="text-yellow-300">&#125;</span></span>
						</div>
					</div>
				</div>
			</a>

			<!-- 2. INTERACTIVE TERMINAL (full width of col-span-2) -->
			<a href="https://elearning.algohub.web.id" target="_blank" rel="noopener"
				class="h-full flex flex-col sm:flex-row items-center gap-6 group overflow-hidden relative rounded-[2rem] p-6 lg:p-8 shadow-lg reveal delay-150 hover:shadow-xl transition-all hover:-translate-y-0.5"
				style="background: linear-gradient(135deg, #1e293b 0%, #111c30 60%, #112240 100%); border: 1px solid #1e3a8a;">
				<div class="absolute inset-0 bg-gradient-to-br from-cyan-950/20 via-slate-900/10 to-transparent pointer-events-none z-0"></div>
				
				<!-- Left: text -->
				<div class="relative z-10 flex-1 text-left min-w-0 self-center">
					<span class="px-2.5 py-0.5 bg-cyan-950 text-cyan-400 rounded-full text-[9px] font-black tracking-wider uppercase border border-cyan-800/30 inline-flex items-center gap-1 mb-3">
						<Terminal size={11} /> Client-side WASM
					</span>
					<h3 class="text-2xl font-extrabold tracking-tight mb-2 text-white group-hover:text-cyan-400 transition-colors">Interactive Terminal</h3>
					<p class="text-slate-300 text-xs font-semibold leading-relaxed mb-4">
						Jalankan program secara instan menggunakan terminal interaktif berbasis compiler client-side WASM.
					</p>
					<div class="inline-flex items-center gap-1 text-xs font-bold text-cyan-400 group-hover:gap-2 transition-all">
						Uji Kode Anda <ArrowRight size={12} />
					</div>
				</div>

				<!-- Right: terminal mockup -->
				<div class="relative z-10 shrink-0 w-full sm:w-64 bg-slate-950 rounded-xl border border-slate-800 shadow-2xl overflow-hidden transform group-hover:-translate-y-1 transition-transform duration-500">
					<div class="bg-slate-900 px-3 py-1.5 flex items-center gap-1.5 border-b border-slate-800">
						<span class="text-[8px] font-mono text-slate-400">Terminal</span>
						<div class="ml-auto flex items-center gap-1 px-1.5 py-0.5 rounded bg-fun-green/20 border border-fun-green/30">
							<span class="text-[7px] font-black text-fun-green uppercase tracking-wider">Online</span>
						</div>
					</div>
					<div class="bg-slate-950 p-3 text-[9px] font-mono text-left min-h-[52px]">
						<div class="text-slate-400 flex items-center">
							<span class="text-fun-green font-bold mr-1.5">$</span>
							<span class="text-slate-300 terminal-input inline-block">./main</span>
						</div>
						<div class="text-fun-green font-bold terminal-output mt-0.5">Halo Lab AP</div>
						<div class="inline-flex items-center gap-1 mt-0.5 terminal-cursor-line">
							<span class="text-fun-green font-bold">$</span>
							<span class="w-1.5 h-3 bg-fun-green/80 rounded-sm animate-pulse inline-block"></span>
						</div>
					</div>
				</div>
			</a>

		</div>

		<!-- 2. JADWAL SHIFT (col 3, natural height) -->
		<div class="relative bg-white rounded-[2rem] border border-blue-100 p-6 lg:p-7 h-full flex flex-col group shadow-sm transition-all hover:border-blue-200 hover:shadow-md reveal delay-200">
			<div class="absolute top-0 left-0 right-0 h-1 rounded-t-[2rem] bg-gradient-to-r from-fun-blue to-blue-600 opacity-0 group-hover:opacity-100 transition-opacity"></div>
			<div class="flex items-center gap-3 mb-6">
				<div class="w-10 h-10 bg-gradient-to-br from-fun-blue to-blue-600 text-white rounded-xl flex items-center justify-center shadow-md shadow-blue-200">
					<Calendar size={20} />
				</div>
				<h3 class="text-lg font-extrabold text-slate-900 tracking-tight">Jadwal Praktikum</h3>
			</div>
			<div class="flex flex-col space-y-3 flex-1">
				{#each shiftJadwal as s}
					{#if s.active}
						<div class="bg-gradient-to-br from-blue-50 to-indigo-50 rounded-2xl p-4 border border-blue-200">
							<div class="flex justify-between items-center mb-1">
								<span class="text-[10px] font-black text-blue-700 bg-white px-2.5 py-0.5 rounded-full uppercase tracking-wider shadow-sm border border-blue-200">Aktif Sekarang</span>
								<span class="text-blue-900 text-xs font-black">{s.kelas?.nama_kelas}</span>
							</div>
							<p class="text-slate-900 text-sm font-extrabold mt-2">{s.hari}</p>
							<p class="text-slate-600 text-xs font-bold mt-0.5">{s.jam_mulai} – {s.jam_selesai}</p>
						</div>
					{:else}
						<div class="flex items-center justify-between p-3 rounded-2xl border border-transparent hover:border-blue-100 hover:bg-blue-50/40 transition-all cursor-default text-left">
							<div>
								<p class="text-slate-900 text-sm font-bold">{s.hari}</p>
								<p class="text-slate-600 text-xs font-semibold">{s.kelas?.nama_kelas} · {s.jam_mulai}</p>
							</div>
							<div class="w-8 h-8 rounded-full bg-slate-50 border border-slate-200 flex items-center justify-center text-slate-500">
								<ArrowRight size={12} class="-rotate-45" />
							</div>
						</div>
					{/if}
				{/each}
				{#if jadwalList.length === 0}
					<p class="text-slate-600 text-xs font-semibold text-center py-4">Memuat jadwal…</p>
				{/if}
			</div>
			<a href="/info/jadwal" class="w-full mt-4 py-2.5 bg-blue-50 hover:bg-blue-100 border border-blue-200 text-sm font-bold text-blue-700 rounded-xl transition-colors text-center">
				Lihat Kalender Lengkap
			</a>
		</div>

	</div><!-- end row 1 -->

	<!-- ROW 2: Modul (col 1) + Pedoman (col 2) + Tim Asisten (col 3) -->
	<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">

		<!-- 4. MODUL BELAJAR -->
		<div class="bg-white rounded-[2rem] border border-emerald-100 p-6 lg:p-7 h-full flex flex-col justify-between group shadow-sm transition-all hover:border-emerald-200 hover:shadow-md reveal delay-300">
			<div class="flex justify-between items-start mb-4">
				<div class="w-10 h-10 bg-gradient-to-br from-fun-green to-emerald-600 text-white rounded-xl flex items-center justify-center shadow-md shadow-emerald-200">
					<Book size={20} />
				</div>
				<span class="bg-emerald-50 text-emerald-800 text-[10px] font-black px-2.5 py-1 rounded-md border border-emerald-200">6 Modul</span>
			</div>
			<div class="text-left">
				<h3 class="text-lg font-extrabold text-slate-900 mb-3">Modul Belajar</h3>
				<ul class="space-y-1.5">
					{#each [
						{ no: 1, label: 'Dasar Logika & Pemrograman C' },
						{ no: 2, label: 'Struktur Kontrol dalam C' },
						{ no: 3, label: 'Array, Struct & Operasi File' },
						{ no: 4, label: 'Pengenalan Bahasa Python' },
						{ no: 5, label: 'Percabangan & Perulangan Python' },
						{ no: 6, label: 'List, Dictionary & File' },
					] as m}
						<li class="flex items-center gap-2 text-[11px] font-bold text-slate-700">
							<span class="w-5 h-5 rounded-full bg-emerald-100 text-emerald-700 text-[9px] font-black flex items-center justify-center shrink-0">{m.no}</span>
							{m.label}
						</li>
					{/each}
				</ul>
			</div>
		</div>

		<!-- 3. PEDOMAN LAPORAN -->
		<div class="bg-white rounded-[2rem] border border-purple-100 p-6 lg:p-7 h-full flex flex-col justify-between group shadow-sm transition-all hover:border-purple-200 hover:shadow-md reveal delay-400">
			<div class="flex justify-between items-start mb-4">
				<div class="w-10 h-10 bg-gradient-to-br from-fun-purple to-indigo-600 text-white rounded-xl flex items-center justify-center shadow-md shadow-purple-200">
					<FileText size={20} />
				</div>
				<span class="bg-red-50 text-red-700 text-[10px] font-black px-2.5 py-1 rounded-md border border-red-200 uppercase">Wajib</span>
			</div>
			<div class="text-left">
				<h3 class="text-lg font-extrabold text-slate-900 mb-1">Pedoman &amp; Lembar Kerja</h3>
				<p class="text-xs text-slate-600 font-semibold leading-relaxed mb-3">Format laporan &amp Lembar Kerja Praktikum.</p>
				<a href="/info/laporan" class="flex items-center justify-between p-2.5 bg-purple-50 rounded-xl border border-purple-100 hover:border-fun-purple/40 hover:bg-white transition-all group/dl">
					<div class="flex items-center gap-2">
						<FileText size={16} class="text-fun-purple shrink-0" />
						<span class="text-xs font-extrabold text-slate-800">Buka Halaman Pedoman</span>
					</div>
					<ArrowRight size={14} class="text-slate-400 group-hover/dl:text-fun-purple transition-colors" />
				</a>
			</div>
		</div>

		<!-- 5. TIM ASISTEN (1 col, vertical layout) -->
		<div class="h-full flex flex-col relative overflow-hidden rounded-[2rem] p-6 lg:p-8 shadow-lg group reveal delay-100"
			style="background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%); border: 1px solid #334155;">
			<div class="absolute -right-8 -bottom-8 w-48 h-48 bg-primary/15 rounded-full blur-3xl pointer-events-none group-hover:bg-primary/25 transition-all duration-500"></div>
			<div class="absolute left-0 top-1/2 w-24 h-24 bg-fun-blue/10 rounded-full blur-2xl pointer-events-none"></div>
			<div class="absolute top-0 left-0 right-0 h-1 bg-gradient-to-r from-fun-green via-fun-blue to-fun-purple rounded-t-[2rem]"></div>
			<div class="relative z-10 flex flex-col h-full gap-5">
				<div class="text-left">
					<div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-slate-800 border border-slate-700 mb-3">
						<span class="w-1.5 h-1.5 rounded-full bg-fun-green shadow-[0_0_8px_rgba(6,214,160,0.7)]"></span>
						<span class="text-[10px] font-black text-slate-200 uppercase tracking-widest">Tim Asisten</span>
					</div>
					<h3 class="text-xl font-extrabold text-white mb-2 leading-tight">Butuh Bantuan Praktikum?</h3>
					<p class="text-sm text-slate-300 font-semibold leading-relaxed">
						{asisten.length > 0 ? asisten.length : '—'} Asisten Laboratorium siap mendampingi proses belajarmu. Jangan ragu untuk bertanya!
					</p>
				</div>
				<div class="flex flex-col items-center gap-3 mt-auto">
					<div class="flex -space-x-3">
						{#each asistenPreview as a, i}
							<div class="relative hover:translate-y-[-4px] transition-transform cursor-pointer" style="z-index: {4-i}">
								{#if a.foto_url}
									<img src={a.foto_url} alt={a.nama} class="w-12 h-12 rounded-full border-2 border-slate-900 shadow-lg object-cover">
								{:else}
									<img src="https://ui-avatars.com/api/?name={encodeURIComponent(a.nama)}&background=0D8ABC&color=fff&bold=true" alt={a.nama} class="w-12 h-12 rounded-full border-2 border-slate-900 shadow-lg">
								{/if}
								<div class="absolute bottom-0 right-0 w-3 h-3 bg-fun-green border-2 border-slate-900 rounded-full"></div>
							</div>
						{/each}
						{#if asistenExtra > 0}
							<div class="relative z-[1] w-12 h-12 rounded-full border-2 border-slate-900 bg-slate-800 flex items-center justify-center text-white font-black text-xs shadow-lg hover:bg-slate-700 transition-colors cursor-pointer">
								+{asistenExtra}
							</div>
						{/if}
					</div>
					<a href="/info/asisten" class="text-xs font-bold text-fun-blue hover:text-sky-300 transition-colors bg-fun-blue/10 px-3 py-1.5 rounded-lg border border-fun-blue/20">
						Lihat Daftar Asisten
					</a>
				</div>
			</div>
		</div>

	</div><!-- end row 2 -->

	</section>

	<!-- ─── TENTANG LABORATORIUM ─── -->
	<section class="pt-16 mt-4 text-left space-y-12 reveal">
		<div class="max-w-2xl">
			<div class="flex items-center gap-3 mb-3">
				<div class="h-1 w-10 rounded-full bg-gradient-to-r from-primary to-fun-purple"></div>
				<span class="text-xs font-black text-slate-600 uppercase tracking-widest">Profil Laboratorium</span>
			</div>
			<h2 class="text-3xl font-extrabold text-slate-900 tracking-tight mb-3">Tentang Laboratorium</h2>
			<p class="text-sm font-semibold text-slate-700 leading-relaxed">
				Laboratorium Algoritma &amp; Pemrograman merupakan pusat riset, praktikum, dan pengembangan skill komputasional bagi mahasiswa. Kami menyediakan sarana terpadu untuk menunjang kegiatan akademis dan kreativitas mahasiswa di bidang pemrograman.
			</p>
		</div>

		<div class="grid grid-cols-1 md:grid-cols-3 gap-6">
			<div class="rounded-2xl border border-primary/15 bg-white p-6 shadow-sm flex flex-col justify-between reveal delay-100 hover:border-primary/30 hover:shadow-md transition-all">
				<div>
					<div class="w-10 h-10 rounded-xl bg-gradient-to-br from-primary to-primary-hover text-white flex items-center justify-center mb-4 shadow-md shadow-primary/20">
						<Compass size={20} />
					</div>
					<h3 class="text-lg font-extrabold text-slate-900 mb-2">Visi &amp; Misi</h3>
					<p class="text-xs text-slate-700 leading-relaxed font-semibold">
						Menjadi laboratorium rujukan dalam pengembangan pemikiran logis dan terstruktur dalam memecahkan permasalahan.
					</p>
				</div>
				<div class="mt-4 pt-4 border-t border-primary/10 flex flex-col gap-1.5 text-[11px] font-bold text-left">
					<span class="text-primary">• Pembelajaran terstruktur</span>
					
					<span class="text-primary">• Pendampingan mahasiswa</span>
				</div>
			</div>

			<div class="rounded-2xl border border-cyan-100 bg-white p-6 shadow-sm reveal delay-200 hover:border-cyan-200 hover:shadow-md transition-all">
				<div class="w-10 h-10 rounded-xl bg-gradient-to-br from-fun-blue to-cyan-500 text-white flex items-center justify-center mb-4 shadow-md shadow-cyan-200">
					<Cpu size={20} />
				</div>
				<h3 class="text-lg font-extrabold text-slate-900 mb-2">Fasilitas Utama</h3>
				<ul class="space-y-3 mt-2 text-xs font-bold text-slate-700 text-left">
					<li class="flex items-center gap-2.5 p-2 rounded-lg bg-cyan-50 border border-cyan-100">
						<MonitorPlay size={16} class="text-fun-blue shrink-0" />
						<span>Fitur Live Code Editor</span>
					</li>
					<li class="flex items-center gap-2.5 p-2 rounded-lg bg-cyan-50 border border-cyan-100">
						<Wifi size={16} class="text-fun-blue shrink-0" />
						<span>Akses Internet Berbasis wifi.id</span>
					</li>
					<li class="flex items-center gap-2.5 p-2 rounded-lg bg-cyan-50 border border-cyan-100">
						<Clock size={16} class="text-fun-blue shrink-0" />
						<span>Ruang Diskusi AC &amp; Smart TV</span>
					</li>
				</ul>
			</div>

			<div class="rounded-2xl border border-amber-100 bg-white p-6 shadow-sm flex flex-col justify-between reveal delay-300 hover:border-amber-200 hover:shadow-md transition-all">
				<div>
					<div class="w-10 h-10 rounded-xl bg-gradient-to-br from-fun-yellow to-orange-400 text-white flex items-center justify-center mb-4 shadow-md shadow-yellow-200">
						<Clock size={20} />
					</div>
					<h3 class="text-lg font-extrabold text-slate-900 mb-3">Jam Operasional</h3>
				</div>
				<div class="space-y-2 text-xs font-bold">
					<p class="text-slate-600 font-semibold mb-3">Laboratorium melayani mahasiswa sesuai jadwal kerja asisten berikut:</p>
					<div class="rounded-xl border border-amber-200 overflow-hidden">
						<div class="flex justify-between items-center bg-amber-50 px-3 py-2 border-b border-amber-100">
							<span class="text-slate-700 font-black">Senin – Jumat</span>
							<span class="text-amber-800 font-black">07:20 – 18:40 WIB</span>
						</div>
						<div class="flex justify-between items-center px-3 py-2 border-b border-amber-100 bg-white">
							<span class="text-slate-600 font-bold">Pagi</span>
							<span class="text-slate-700 font-bold">07:20 – 12:00</span>
						</div>
						<div class="flex justify-between items-center px-3 py-2 bg-white">
							<span class="text-slate-600 font-bold">Siang</span>
							<span class="text-slate-700 font-bold">13:00 – 18:40</span>
						</div>
					</div>
					<div class="flex justify-between items-center bg-slate-50 rounded-xl px-3 py-2 border border-slate-100">
						<span class="text-slate-500 font-bold">Sabtu – Minggu</span>
						<span class="text-slate-500 font-bold italic">Tutup</span>
					</div>
				</div>
			</div>
		</div>

		<!-- Contact + Map -->
		<div class="grid grid-cols-1 md:grid-cols-12 gap-6 items-stretch">
			<div class="md:col-span-5 flex flex-col justify-between bg-white border border-slate-200 rounded-[2rem] p-6 lg:p-8 shadow-sm reveal delay-100 hover:border-slate-300 hover:shadow-md transition-all">
				<div class="space-y-6">
					<div class="flex items-center gap-3 mb-1">
						<div class="h-1 w-6 rounded-full bg-gradient-to-r from-primary to-fun-purple"></div>
						<h3 class="text-xl font-extrabold text-slate-900">Informasi Kontak</h3>
					</div>
					<div class="space-y-4 text-left">
						<div class="flex items-start gap-3">
							<div class="w-8 h-8 rounded-lg bg-gradient-to-br from-primary/10 to-fun-purple/10 border border-primary/20 flex items-center justify-center shrink-0 mt-0.5 text-primary"><Building2 size={16} /></div>
							<div>
								<h4 class="text-xs font-black text-slate-600 uppercase tracking-wider">Kampus</h4>
								<p class="text-xs font-semibold text-slate-800 leading-relaxed mt-0.5"><strong>Menara PLN</strong><br/>Jl. Lkr. Luar Barat Lantai 2, RT.1/RW.1, Duri Kosambi, Cengkareng, Jakarta Barat 11750.</p>
							</div>
						</div>
						<div class="flex items-start gap-3 border-t border-slate-100 pt-4">
							<div class="w-8 h-8 rounded-lg bg-gradient-to-br from-fun-blue/10 to-cyan-100 border border-fun-blue/20 flex items-center justify-center shrink-0 mt-0.5 text-fun-blue"><MapPin size={16} /></div>
							<div>
								<h4 class="text-xs font-black text-slate-600 uppercase tracking-wider">Laboratorium</h4>
								<p class="text-xs font-semibold text-slate-800 leading-relaxed mt-0.5"><strong>Smart Electronic Systems Laboratory</strong><br/>Gedung C lantai 2, Institut Teknologi PLN.</p>
							</div>
						</div>
						<div class="flex items-start gap-3 border-t border-slate-100 pt-4">
							<div class="w-8 h-8 rounded-lg bg-gradient-to-br from-fun-green/10 to-emerald-100 border border-fun-green/20 flex items-center justify-center shrink-0 mt-0.5 text-fun-green"><Mail size={16} /></div>
							<div>
								<h4 class="text-xs font-black text-slate-600 uppercase tracking-wider">Email</h4>
								<p class="text-sm font-bold text-slate-800 mt-0.5">labalgoritmapemrograman@gmail.com</p>
							</div>
						</div>
					</div>
				</div>
				<div class="mt-8 pt-6 border-t border-slate-100 flex items-center justify-between">
					<span class="text-xs font-black text-slate-600">Media Sosial Resmi</span>
					<a href="https://www.instagram.com/lab_ap.itpln?igsh=cjFocGp5MDB5N280" target="_blank" class="w-8 h-8 rounded-full bg-gradient-to-br from-pink-400 to-purple-500 flex items-center justify-center text-white shadow-sm hover:scale-110 transition-transform">
						<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="20" x="2" y="2" rx="5" ry="5"/><path d="M16 11.37A4 4 0 1 1 12.63 8 4 4 0 0 1 16 11.37z"/><line x1="17.5" x2="17.51" y1="6.5" y2="6.5"/></svg>
					</a>
				</div>
			</div>

			<div class="md:col-span-7 rounded-[2rem] border border-slate-200 shadow-sm relative overflow-hidden group min-h-[300px] flex flex-col justify-end reveal delay-200" style="background:#e2e8f0;">
				<iframe title="Google Maps ITPLN Lab AP" src="https://maps.google.com/maps?q=-6.168200,%20106.725385&t=&z=17&ie=UTF8&iwloc=&output=embed" class="absolute inset-0 w-full h-full border-0 z-0 grayscale opacity-85 group-hover:grayscale-0 group-hover:opacity-100 transition-all duration-500" allowfullscreen={true} loading="lazy" referrerpolicy="no-referrer-when-downgrade"></iframe>
				<div class="relative z-10 m-4 p-4 bg-white/95 backdrop-blur-sm border border-slate-200 rounded-2xl shadow-lg flex items-center justify-between text-left">
					<div>
						<h4 class="text-sm font-extrabold text-slate-900">Laboratorium Algoritma &amp; Pemrograman</h4>
						<p class="text-[11px] font-bold text-slate-600 mt-0.5">Gedung Kampus ITPLN, Cengkareng, Jakarta Barat</p>
					</div>
					<a href="https://maps.app.goo.gl/GubrGj3VyUjCTDLo8" target="_blank"
						class="px-4 py-2 bg-gradient-to-r from-primary to-primary-hover text-white rounded-xl text-xs font-bold shadow-sm transition-all hover:brightness-110 shrink-0">
						Buka Google Maps
					</a>
				</div>
			</div>
		</div>

	</section>

	<!-- Plagiarism List Modal Overlay -->
	{#if showPlagiarismModal}
		<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm select-none" onclick={() => showPlagiarismModal = false}>
			<div class="bg-white rounded-3xl max-w-xl w-full border border-slate-200 shadow-2xl overflow-hidden transform transition-all flex flex-col max-h-[85vh] relative" onclick={(e) => e.stopPropagation()}>
				<!-- Top alert line -->
				<div class="h-1.5 w-full bg-gradient-to-r from-red-500 to-rose-600"></div>

				<!-- Header -->
				<div class="p-6 border-b border-slate-100 flex items-start justify-between gap-4">
					<div>
						<h3 class="text-lg font-black text-slate-900 flex items-center gap-2">
							<AlertTriangle size={18} class="text-red-500" />
							Daftar Pelanggaran Plagiarisme
						</h3>
						<p class="text-xs text-slate-500 font-semibold mt-1">Daftar praktikan yang terdeteksi melakukan tindakan plagiarisme.</p>
					</div>
					<button onclick={() => showPlagiarismModal = false} class="w-8 h-8 rounded-full hover:bg-slate-100 flex items-center justify-center text-slate-400 hover:text-slate-600 transition-colors">
						<X size={16} />
					</button>
				</div>

				<!-- Body -->
				<div class="p-6 overflow-y-auto max-h-[50vh]">
					<table class="w-full text-left border-collapse">
						<thead>
							<tr class="border-b border-slate-200 text-[10px] font-black uppercase text-slate-400 tracking-wider">
								<th class="pb-3 pr-4">NIM</th>
								<th class="pb-3 pr-4">Nama</th>
								<th class="pb-3 pr-4">Persentase</th>
								<th class="pb-3 pr-4">Hari/Tanggal</th>
								<th class="pb-3">Jam</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-slate-100 text-xs font-semibold text-slate-700">
							{#each plagiarismData.rows as row}
								{@const pct = parseInt((row.keterangan || '').replace(/[^0-9]/g, ''), 10)}
								<tr>
									<td class="py-3.5 pr-4 font-mono text-slate-500">{row.nim}</td>
									<td class="py-3.5 pr-4 text-slate-900 font-extrabold">{row.nama}</td>
									<td class="py-3.5 pr-4 {isNaN(pct) ? 'text-slate-700 font-semibold' : pct >= 70 ? 'text-red-600 font-extrabold' : pct >= 50 ? 'text-amber-500 font-extrabold' : 'text-emerald-600 font-semibold'}">{row.keterangan || '—'}</td>
									<td class="py-3.5 pr-4">
										{#if row.hari}
											<span class="px-2.5 py-0.5 bg-slate-100 text-slate-800 border border-slate-200/80 rounded-md font-semibold text-[11px] inline-block shadow-sm">{row.hari}</span>
										{:else}
											<span class="text-slate-400">—</span>
										{/if}
									</td>
									<td class="py-3.5">
										{#if row.jam}
											<span class="px-2.5 py-0.5 bg-rose-50 text-rose-800 border border-rose-100 rounded-md font-semibold text-[11px] inline-block shadow-sm">{row.jam}</span>
										{:else}
											<span class="text-slate-400">—</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>

					<!-- Callout Instruksi Kehadiran -->
					<div class="mt-5 p-4 rounded-2xl bg-rose-50 border border-rose-100 text-xs text-rose-800 leading-relaxed font-semibold flex items-start gap-2.5 shadow-sm">
						<AlertTriangle size={16} class="text-rose-600 shrink-0 mt-0.5" />
						<p>{plagiarismData.catatan || "Bagi nama-nama yang disebutkan di atas, silakan hadir di Lab. Algoritma & Pemrograman pada Rabu, 08 Juli 2026 jam 08:00 Pagi."}</p>
					</div>
				</div>

				<!-- Footer -->
				<div class="p-4 bg-slate-50 border-t border-slate-100 flex justify-between items-center text-[10px] text-slate-500 font-bold">
					<span>Pembaruan Terakhir: {plagiarismData.updatedAt || '—'}</span>
					<button onclick={() => showPlagiarismModal = false} class="px-4 py-2 bg-slate-900 hover:bg-slate-800 text-white rounded-xl text-xs font-bold transition-colors">
						Tutup
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
/* Typewriter animations for terminal */
.terminal-input {
	overflow: hidden;
	white-space: nowrap;
	width: 0;
	animation: loop-input 4s infinite;
}

.terminal-output {
	overflow: hidden;
	white-space: nowrap;
	width: 0;
	animation: loop-output 4s infinite;
}

.terminal-cursor-line {
	animation: loop-cursor 4s infinite;
}

@keyframes loop-input {
	0%, 10% { width: 0; }
	25%, 85% { width: 6ch; }
	95%, 100% { width: 0; }
}

@keyframes loop-output {
	0%, 30% { width: 0; opacity: 0; }
	35% { opacity: 1; }
	50%, 85% { width: 11ch; opacity: 1; }
	95%, 100% { width: 0; opacity: 0; }
}

@keyframes loop-cursor {
	0%, 50% { opacity: 0; }
	55%, 85% { opacity: 1; }
	95%, 100% { opacity: 0; }
}

/* Respect reduced-motion — just show the text instantly */
@media (prefers-reduced-motion: reduce) {
	.terminal-input {
		animation: none;
		width: auto;
	}
	.terminal-output {
		animation: none;
		width: auto;
	}
	.terminal-cursor-line {
		animation: none;
		opacity: 1;
	}
}
</style>
