import { get } from 'svelte/store';
import { cSabStore } from './stores/runner';

// Tulis input user ke SharedArrayBuffer lalu notify worker C (mode blocking stdin).
// Return true kalau berhasil, false kalau SAB tidak tersedia.
export function sendCInput(value: string): boolean {
	const sab = get(cSabStore);
	if (!sab) return false;
	try {
		const meta = new Int32Array(sab, 0, 2);
		const data = new Uint8Array(sab, 8);
		const bytes = new TextEncoder().encode(value + '\n');
		if (bytes.length > data.length) return false;
		data.set(bytes);
		Atomics.store(meta, 1, bytes.length);
		Atomics.store(meta, 0, 1);
		Atomics.notify(meta, 0);
		return true;
	} catch {
		return false;
	}
}

// Batalkan menunggu input di worker (dipakai Reset/Stop). Flag 2 = cancel.
export function cancelCInput(): void {
	const sab = get(cSabStore);
	if (!sab) return;
	try {
		const meta = new Int32Array(sab, 0, 2);
		Atomics.store(meta, 0, 2);
		Atomics.notify(meta, 0);
	} catch {
		/* abaikan */
	}
}
