package usecase

import (
	"testing"
)

func TestKodeBersih_AmbilBlokTerbesar(t *testing.T) {
	kunci := `<p>Acuan penilaian</p><pre><code class="language-c">   #include &lt;stdio.h&gt;</code></pre>` +
		`<pre><code class="language-c">#include &lt;stdio.h&gt;\n\nint main() {\n    printf("halo");\n    return 0;\n}</code></pre>`
	got := kodeBersih(kunci)
	if got == "" || !contains(got, "int main()") {
		t.Fatalf("harap blok main, dapat: %q", got)
	}
	if contains(got, "<") && contains(got, "&lt;") {
		t.Fatalf("entity HTML harus ter-decode: %q", got)
	}
}

func TestKodeBersih_TanpaBlok(t *testing.T) {
	if got := kodeBersih(`<p>cuma teks</p>`); got != "" {
		t.Fatalf("harap kosong, dapat %q", got)
	}
}

func TestInstruksiBersih_StripHTML(t *testing.T) {
	got := instruksiBersih(`<p>Buat <code>main()</code> &amp; cetak</p>`)
	if got != "Buat main() & cetak" {
		t.Fatalf("harap teks polos, dapat %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
