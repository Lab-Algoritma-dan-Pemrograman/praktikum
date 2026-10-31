package cwasm

import (
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSemaphoreLimitsConcurrency membuktikan antrean membatasi compile bersamaan.
// Tanpa semaphore, 20 goroutine akan jalan serentak; dengan limit 2, maksimal 2 aktif.
func TestSemaphoreLimitsConcurrency(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh tidak tersedia")
	}

	c := New("sh", "/tmp").WithLimits(2, 10*time.Second)

	var active int32
	var peak int32
	var wg sync.WaitGroup

	const n = 20
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem := c.acquire()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-time.After(c.queueTO):
				t.Error("terjadi timeout antrean yang tak diharapkan")
				return
			}

			cur := atomic.AddInt32(&active, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&active, -1)
		}()
	}
	wg.Wait()

	if peak > 2 {
		t.Fatalf("konkurensi melebihi batas: peak=%d, batas=2", peak)
	}
	t.Logf("peak konkurensi = %d (batas 2) ✓", peak)
}

// TestQueueTimeout: antrean penuh -> kembalikan pesan sibuk, bukan menggantung.
func TestQueueTimeout(t *testing.T) {
	c := New("sh", "/tmp").WithLimits(1, 100*time.Millisecond)
	sem := c.acquire()

	sem <- struct{}{} // isi satu-satunya slot

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Compile akan antre; karena slot penuh & timeout 100ms, harus cepat balik.
		_, msg, err := c.Compile("int main(){}")
		if err != nil {
			t.Errorf("error tak terduga: %v", err)
		}
		if msg == "" {
			t.Error("harus mengembalikan pesan sibuk")
		} else {
			t.Logf("pesan: %q", msg)
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Compile menggantung; queue timeout tidak bekerja")
	}
}

// TestWithLimitsDefaults: nilai 0 tidak meng-override bawaan.
func TestWithLimitsDefaults(t *testing.T) {
	c := New("clang", "/sysroot")
	c.WithLimits(0, 0)
	if c.maxConc != 4 {
		t.Errorf("maxConc = %d, ingin 4", c.maxConc)
	}
	if c.queueTO != 30*time.Second {
		t.Errorf("queueTO = %v, ingin 30s", c.queueTO)
	}
}
