// CR-H2: sanitasi HTML terpusat untuk semua titik {@html}.
// Teks soal/flowchart berasal dari input admin/AI → harus lewat DOMPurify
// (allowlist) sebelum dirender, bukan regex blacklist.
import DOMPurify from 'dompurify';

const SOAL_TAGS = [
	'div', 'pre', 'code', 'span', 'p', 'br', 'h1', 'h2', 'h3', 'h4',
	'ul', 'ol', 'li', 'strong', 'em', 'u', 'table', 'thead', 'tbody',
	'tr', 'td', 'th', 'a', 'img', 'blockquote', 'sub', 'sup', 'hr'
];

const SOAL_ATTR = [
	'class', 'href', 'src', 'alt', 'title', 'target', 'rel',
	'colspan', 'rowspan', 'width', 'height'
];

/** Sanitasi HTML konten soal/modul sebelum {@html}. */
export function sanitizeSoal(html: string | null | undefined): string {
	if (!html) return '';
	return DOMPurify.sanitize(html, {
		ALLOWED_TAGS: SOAL_TAGS,
		ALLOWED_ATTR: SOAL_ATTR,
		ALLOW_DATA_ATTR: false,
		FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form', 'input']
	});
}

/** Escape HTML penuh untuk teks polos (pesan konfirmasi, nama, NIM). */
export function escapeHtml(text: string | null | undefined): string {
	if (!text) return '';
	return text.replace(/[&<>"']/g, (ch) => {
		switch (ch) {
			case '&': return '&amp;';
			case '<': return '&lt;';
			case '>': return '&gt;';
			case '"': return '&quot;';
			case "'": return '&#39;';
			default: return ch;
		}
	});
}
