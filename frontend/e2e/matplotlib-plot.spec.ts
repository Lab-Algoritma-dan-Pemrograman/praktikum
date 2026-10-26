import { test, expect } from '@playwright/test';

// Uji matplotlib end-to-end lewat UI nyata: ketik kode -> Run -> grafik tampil.
// Timeout besar: Monaco + Pyodide + wheel matplotlib (~37 MB) diunduh saat run pertama.
test.setTimeout(360_000);

test('matplotlib menghasilkan grafik di panel terminal', async ({ page }) => {
	await page.goto('/info/praktikum');

	// Pilih Python
	await page.getByRole('button', { name: 'Python', exact: true }).click();

	const editor = page.locator('.monaco-editor').first();
	await editor.waitFor({ state: 'visible', timeout: 60_000 });
	await editor.click();
	await page.keyboard.press('Control+A');
	await page.keyboard.insertText(`import matplotlib.pyplot as plt

plt.plot([1, 2, 3, 4], [5, 10, 15, 20])
plt.title("Grafik Perkembangan")
plt.xlabel("Waktu")
plt.ylabel("Nilai")
plt.grid(True)
plt.show()
print("Konfigurasi grafik selesai!")
`);

	await page.getByRole('button', { name: /Run/ }).click();

	// Init Pyodide + unduh matplotlib bisa lama.
	const img = page.locator('img[alt^="Grafik matplotlib"]');
	await expect(img).toBeVisible({ timeout: 300_000 });

	const src = await img.getAttribute('src');
	expect(src?.startsWith('data:image/png;base64,')).toBeTruthy();
	// fetch(data:) diblok CSP connect-src, jadi ukuran dihitung dari base64.
	const b64 = src!.split(',')[1];
	expect(Math.floor((b64.length * 3) / 4)).toBeGreaterThan(5000);

	await expect(page.locator('.sr-only[aria-live="polite"]')).toContainText(
		'Konfigurasi grafik selesai!'
	);
});
