package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

type rumpangFakeGen struct{ reply string }

func (f rumpangFakeGen) Complete(_ context.Context, _, _ string) (string, error) {
	return f.reply, nil
}

func TestGenerateRumpangFillsStarterCode(t *testing.T) {
	h := newPracticumHarness(t, "keterampilan", "individual", false)
	// Harness coding question already has StarterCode; create one without.
	q, err := h.s.SaveQuestion(t.Context(), h.admin, practicumCmd("rumpang-q"), "", PracticumQuestionUpdate{PracticumQuestionInput: PracticumQuestionInput{
		CourseID: h.course.ID, Type: "coding", Difficulty: "easy", Prompt: "<p>Cetak nama</p>", Points: 10,
		Language: "c", ReferenceAnswer: `<pre><code class="language-c">#include &lt;stdio.h&gt;
int main(void) { char nama[64]; printf("Masukkan nama: "); scanf("%s", nama); printf("Hai %s\n", nama); return 0; }</code></pre>`,
		Rubric: []entity.Criterion{{ID: "logic", Description: "benar", MaxScore: 10}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	h.s.options.Rumpang = rumpangFakeGen{reply: "```\n#include <stdio.h>\nint main(void) { __ nama[64]; ____(\"Masukkan nama: \"); return 0; }\n```"}
	out, err := h.s.GenerateRumpang(t.Context(), h.admin, practicumCmd("rumpang-gen"), q.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.StarterCode, "__") || strings.Contains(out.StarterCode, "```") || strings.HasPrefix(out.StarterCode, "\n") {
		t.Fatalf("starter not sanitized: %q", out.StarterCode)
	}
	if out.Version != 2 {
		t.Fatalf("version = %d, want 2", out.Version)
	}
	// Idempotency guard: second run must refuse, not overwrite.
	if _, err := h.s.GenerateRumpang(t.Context(), h.admin, practicumCmd("rumpang-gen-2"), q.ID); err == nil {
		t.Fatal("expected refusal on existing starter code")
	}
	// Students must never see the reference answer via the attempt view.
	a := h.start(t, 0)
	for _, attempt := range a.Questions {
		if attempt.ReferenceAnswer != "" {
			t.Fatal("reference answer leaked into attempt view")
		}
	}
}

func TestGenerateRumpangRejectsNonCodingOrConfigured(t *testing.T) {
	h := newPracticumHarness(t, "keterampilan", "individual", false)
	h.s.options.Rumpang = rumpangFakeGen{reply: "x"}
	// Essay question: rejected.
	if _, err := h.s.GenerateRumpang(t.Context(), h.admin, practicumCmd("r1"), h.questions[0].ID); err == nil {
		t.Fatal("essay must be rejected")
	}
	// Coding question without starter already has one in the harness: rejected.
	if _, err := h.s.GenerateRumpang(t.Context(), h.admin, practicumCmd("r2"), h.questions[1].ID); err == nil {
		t.Fatal("existing starter must be rejected")
	}
	// Unconfigured generator: rejected.
	h.s.options.Rumpang = nil
	if _, err := h.s.GenerateRumpang(t.Context(), h.admin, practicumCmd("r3"), h.questions[1].ID); err == nil {
		t.Fatal("nil generator must be rejected")
	}
}

func TestRumpangCodeExtractsLargestCBlock(t *testing.T) {
	html := `<p>intro</p><pre><code class="language-c">short</code></pre><pre><code>int main(void) { return 0; }</code></pre>`
	if got := rumpangCode(html); !strings.Contains(got, "int main") {
		t.Fatalf("largest block = %q", got)
	}
	if got := rumpangCode("<p>no code</p>"); got != "" {
		t.Fatalf("no-code = %q", got)
	}
}
