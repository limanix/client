package guest

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestApplyEventsPreservesFragmentedInput(t *testing.T) {
	const output = `{"event":"output","line":"building"}` + "\n"
	const answer = `{"contract":1,"ok":true,"data":{"state":"restart_required"}}`
	input := output + answer
	for _, size := range []int{1, 7, len(output), len(input)} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			var stderr bytes.Buffer
			events := &applyEvents{guest: &Guest{Stderr: &stderr}}
			for offset := 0; offset < len(input); offset += size {
				chunk := []byte(input[offset:min(offset+size, len(input))])
				if n, err := events.Write(chunk); err != nil || n != len(chunk) {
					t.Fatalf("Write = %d, %v", n, err)
				}
				// Writers must not retain the caller's buffer after Write returns.
				clear(chunk)
			}
			if err := events.outcome(nil); err != nil {
				t.Fatal(err)
			}
			if stderr.String() != "building\n" || string(events.answer) != answer {
				t.Fatalf("output = %q, answer = %q", stderr.String(), events.answer)
			}
		})
	}
}

func BenchmarkApplyEventsWrite(b *testing.B) {
	line := `{"event":"output","line":"` + strings.Repeat("building ", 24) + `"}` + "\n"
	for _, lines := range []int{1, 32} {
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			data := []byte(strings.Repeat(line, lines))
			events := &applyEvents{guest: &Guest{Stderr: io.Discard}}
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for b.Loop() {
				_, _ = events.Write(data)
			}
		})
	}
}
