package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFieldsSpec(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		spec    string
		want    []interval
		wantErr bool
	}{
		{"single", "1", []interval{{start: 1, end: 1}}, false},
		{"mix", "1,3-5", []interval{{start: 1, end: 1}, {start: 3, end: 5}}, false},
		{"mergeOverlap", "1,2-3,3-4", []interval{{start: 1, end: 4}}, false},
		{"mergeAdjacent", "1,2,3", []interval{{start: 1, end: 3}}, false},
		{"trimSpaces", " 1 ,  3 - 4 ", []interval{{start: 1, end: 1}, {start: 3, end: 4}}, false},
		{"badEmpty", "1,,2", nil, true},
		{"badZero", "0", nil, true},
		{"badRangeOrder", "3-1", nil, true},
		{"badToken", "a", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFieldsSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len=%d want=%d got=%v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d]=%v want[%d]=%v", i, got[i], i, tt.want[i])
				}
			}
		})
	}
}

func TestParseDelimiter(t *testing.T) {
	t.Parallel()
	_, _, _, err := parseDelimiter("")
	if err == nil {
		t.Fatalf("expected error for empty delimiter")
	}
	_, _, _, err = parseDelimiter("ab")
	if err == nil {
		t.Fatalf("expected error for multi-rune delimiter")
	}
	d, b, single, err := parseDelimiter("\t")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d != "\t" || !single || b != '\t' {
		t.Fatalf("unexpected delimiter parse result: %q %v %v", d, b, single)
	}
}

func TestRun_BasicTab(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("a\tb\tc\n1\t2\t3\n")
	var out, errOut bytes.Buffer
	err := run([]string{"-f", "1,3"}, in, &out, &errOut)
	if err != nil {
		t.Fatalf("run err: %v stderr=%q", err, errOut.String())
	}
	want := "a\tc\n1\t3\n"
	if out.String() != want {
		t.Fatalf("out=%q want=%q", out.String(), want)
	}
}

func TestRun_DelimiterComma(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("a,b,c\nx,y\n")
	var out, errOut bytes.Buffer
	err := run([]string{"-f", "2-3", "-d", ","}, in, &out, &errOut)
	if err != nil {
		t.Fatalf("run err: %v stderr=%q", err, errOut.String())
	}
	want := "b,c\ny\n"
	if out.String() != want {
		t.Fatalf("out=%q want=%q", out.String(), want)
	}
}

func TestRun_SeparatedOnly(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("no_delim\nhas\tdelim\n")
	var out, errOut bytes.Buffer
	err := run([]string{"-f", "1", "-s"}, in, &out, &errOut)
	if err != nil {
		t.Fatalf("run err: %v stderr=%q", err, errOut.String())
	}
	want := "has\n"
	if out.String() != want {
		t.Fatalf("out=%q want=%q", out.String(), want)
	}
}

func TestRun_OutOfRangeFieldsProduceEmptyLine(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("a\tb\n")
	var out, errOut bytes.Buffer
	err := run([]string{"-f", "5"}, in, &out, &errOut)
	if err != nil {
		t.Fatalf("run err: %v stderr=%q", err, errOut.String())
	}
	want := "\n"
	if out.String() != want {
		t.Fatalf("out=%q want=%q", out.String(), want)
	}
}

