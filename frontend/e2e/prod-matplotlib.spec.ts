import { test, expect } from '@playwright/test';

// Verifikasi produksi: grafik matplotlib benar-benar tampil di algohub.web.id
test.setTimeout(420_000);

test('PROD algohub: matplotlib tampil sebagai grafik', async ({ page }) => {
	await page.goto('https://algohub.web.id/info/praktikum', { waitUntil: 'domcontentloaded' });
	await page.getByRole('button', { name: 'Python', exact: true }).click();

	const editor = page.locator('.monaco-editor').first();
	await editor.waitFor({ state: 'visible', timeout: 90_000 });
	await editor.click();
	await page.keyboard.press('Control+A');
	await page.keyboard.insertText(`import matplotlib.pyplot as plt

plt.plot([1, 2, 3, 4], [5, 10, 15, 20])
plt.title("Grafik Perkembangan")
plt.xlabel("Waktu")
plt.ylabel("Nilai")
plt.grid(True)
plt.show()
print("selesai")
`);

	await page.getByRole('button', { name: /Run/ }).click();

	const img = page.locator('img[alt^="Grafik matplotlib"]');
	await expect(img).toBeVisible({ timeout: 360_000 });
	const src = await img.getAttribute('src');
	expect(src?.startsWith('data:image/png;base64,')).toBeTruthy();
	const b64len = src!.split(',')[1].length;
	const info = await img.evaluate((el: HTMLImageElement) => [el.naturalWidth, el.naturalHeight]);
	console.log('PROD_IMG_B64LEN', b64len, 'NATURAL', JSON.stringify(info));
	expect(b64len).toBeGreaterThan(5000);
	await expect(page.locator('.sr-only[aria-live="polite"]')).toContainText('selesai');
});
