package main

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSortLines(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		numeric  bool
		month    bool
		human    bool
		reverse  bool
		blanks   bool
		column   int
		expected []string
	}{
		{
			name:     "базовая сортировка",
			input:    []string{"c", "a", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "обратная сортировка",
			input:    []string{"a", "b", "c"},
			reverse:  true,
			expected: []string{"c", "b", "a"},
		},
		{
			name:     "числовая сортировка",
			input:    []string{"10", "2", "5", "1"},
			numeric:  true,
			expected: []string{"1", "2", "5", "10"},
		},
		{
			name:     "числовая с обратным порядком",
			input:    []string{"10", "2", "5", "1"},
			numeric:  true,
			reverse:  true,
			expected: []string{"10", "5", "2", "1"},
		},
		{
			name:     "сортировка по месяцам",
			input:    []string{"Feb", "Jan", "Mar", "Dec"},
			month:    true,
			expected: []string{"Jan", "Feb", "Mar", "Dec"},
		},
		{
			name:     "смешанные данные с месяцами",
			input:    []string{"apple", "Feb", "banana", "Jan"},
			month:    true,
			expected: []string{"Jan", "Feb", "apple", "banana"},
		},
		{
			name:     "человекочитаемые размеры",
			input:    []string{"1K", "2M", "500", "1G"},
			human:    true,
			expected: []string{"500", "1K", "2M", "1G"},
		},
		{
			name:     "сортировка по колонке (разделитель — табуляция)",
			input:    []string{"b\t2", "a\t1", "c\t3"},
			column:   2,
			numeric:  true,
			expected: []string{"a\t1", "b\t2", "c\t3"},
		},
		{
			name:     "игнорирование пробелов",
			input:    []string{"a  ", "b", "c"},
			blanks:   true,
			expected: []string{"a  ", "b", "c"},
		},
		{
			name:     "числа с единицами измерения",
			input:    []string{"10MB", "5KB", "1GB"},
			human:    true,
			expected: []string{"5KB", "10MB", "1GB"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]string, len(tt.input))
			copy(result, tt.input)

			sortLines(result, tt.numeric, tt.month, tt.human,
				tt.reverse, tt.blanks, tt.column)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ожидалось %v, получено %v", tt.expected, result)
			}
		})
	}
}

func TestUniqueLines(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "без дубликатов",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "с дубликатами",
			input:    []string{"a", "a", "b", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "пустой срез",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueLines(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ожидалось %v, получено %v", tt.expected, result)
			}
		})
	}
}

// TestSortAndUnique проверяет связку sortLines + uniqueLines (как в main при -u).
func TestSortAndUnique(t *testing.T) {
	lines := []string{"b", "a", "b", "c", "a"}
	sortLines(lines, false, false, false, false, false, 0)
	lines = uniqueLines(lines)
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("после sort+unique ожидалось %v, получено %v", expected, lines)
	}
}

func TestParseHumanNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"1K", 1024},
		{"2M", 2 * 1024 * 1024},
		{"3G", 3 * 1024 * 1024 * 1024},
		{"500", 500},
		{"1.5K", 1.5 * 1024},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseHumanNumber(tt.input)
			if result != tt.expected {
				t.Errorf("%s: ожидалось %v, получено %v", tt.input, tt.expected, result)
			}
		})
	}
}

// TestSorted проверяет логику проверки порядка без перехвата stdout.
func TestSorted(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected bool
	}{
		{"пустой", []string{}, true},
		{"одна строка", []string{"a"}, true},
		{"отсортировано", []string{"a", "b", "c"}, true},
		{"не отсортировано", []string{"b", "a", "c"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sorted(tt.lines, false, false, false, false, 0)
			if got != tt.expected {
				t.Errorf("sorted() = %v, ожидалось %v", got, tt.expected)
			}
		})
	}
}

func TestCheckSorted(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	checkSorted([]string{"a", "b", "c"}, false, false, false, false, 1)
	w.Close()
	os.Stdout = old
	if buf, _ := io.ReadAll(r); !strings.Contains(string(buf), "отсортирован") {
		t.Error("для отсортированных данных должно быть 'отсортирован'")
	}

	r, w, _ = os.Pipe()
	os.Stdout = w
	checkSorted([]string{"b", "a", "c"}, false, false, false, false, 1)
	w.Close()
	os.Stdout = old
	if buf, _ := io.ReadAll(r); !strings.Contains(string(buf), "не отсортирован") {
		t.Error("для неотсортированных данных должно быть 'не отсортирован'")
	}
}
