package config

import "testing"

func BenchmarkRenderReference(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RenderReference(); err != nil {
			b.Fatal(err)
		}
	}
}
