package timescale

import "testing"

func TestParseVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in            string
		major, minor, patch int
		wantErr       bool
	}{
		{"2.17.2", 2, 17, 2, false},
		{"2.14.2-dev", 2, 14, 2, false},
		{"1.7.5", 1, 7, 5, false},
		{"3.0.0", 3, 0, 0, false},
		{"", 0, 0, 0, true},
		{"abc", 0, 0, 0, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			major, minor, patch, err := ParseVersion(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", tc.in, err)
			}
			if major != tc.major || minor != tc.minor || patch != tc.patch {
				t.Fatalf("got %d.%d.%d want %d.%d.%d", major, minor, patch, tc.major, tc.minor, tc.patch)
			}
		})
	}
}

func TestEvaluateCompatibility(t *testing.T) {
	t.Parallel()
	ok, note := EvaluateCompatibility(2, "2.17.2")
	if !ok || note != "" {
		t.Fatalf("expected compatible: ok=%v note=%q", ok, note)
	}
	ok, note = EvaluateCompatibility(1, "1.7.5")
	if ok || note == "" {
		t.Fatalf("expected unsupported: ok=%v note=%q", ok, note)
	}
}

func TestAggregateChunkStats(t *testing.T) {
	t.Parallel()
	chunks := []ChunkFacts{
		{SchemaName: "public", HypertableName: "metrics", TotalSizeBytes: 100, IsCompressed: true},
		{SchemaName: "public", HypertableName: "metrics", TotalSizeBytes: 50, IsCompressed: false},
		{SchemaName: "iot", HypertableName: "events", TotalSizeBytes: 10, IsCompressed: false},
	}
	stats := AggregateChunkStats(chunks)
	m := stats["public.metrics"]
	if m.Count != 2 || m.Compressed != 1 || m.TotalSizeBytes != 150 {
		t.Fatalf("metrics stats = %+v", m)
	}
}
