package tests

import (
	"os"
	"testing"

	"github.com/israelsodano/binder"
)

func Benchmark_Bind(t *testing.B) {
	t.StartTimer()
	s, err := os.ReadFile("./source.json")
	if err != nil {
		panic(err)
	}
	d, err := os.ReadFile("./data.json")
	if err != nil {
		panic(err)
	}
	v := binder.Bind(s, d)
	os.WriteFile("out.json", v, 0777)
}
