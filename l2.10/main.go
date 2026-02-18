package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

var (
	column       = flag.Int("k", 0, "колонка N (0=вся строка, табуляция)")
	numeric      = flag.Bool("n", false, "числовая сортировка")
	reverse      = flag.Bool("r", false, "обратный порядок")
	unique       = flag.Bool("u", false, "только уникальные")
	monthSort    = flag.Bool("M", false, "по месяцу (Jan..Dec)")
	ignoreBlanks = flag.Bool("b", false, "игнорировать хвостовые пробелы")
	check        = flag.Bool("c", false, "только проверить порядок")
	humanSort    = flag.Bool("h", false, "размеры 1K, 2M, 1G")
)

var months = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var humanMult = map[byte]float64{
	'K': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024, 'T': 1024 * 1024 * 1024 * 1024,
}

func main() {
	flag.Parse()
	lines := readLines(flag.Args())
	if len(lines) == 0 {
		return
	}
	if *check {
		if !sorted(lines, *numeric, *monthSort, *humanSort, *ignoreBlanks, *column) {
			fmt.Println("файл не отсортирован")
			os.Exit(1)
		}
		fmt.Println("файл отсортирован")
		return
	}
	sortLines(lines, *numeric, *monthSort, *humanSort, *reverse, *ignoreBlanks, *column)
	if *unique {
		lines = uniqueLines(lines)
	}
	for _, s := range lines {
		fmt.Println(s)
	}
}

// readLines читает строки из списка файлов или из stdin (если список пустой).
func readLines(files []string) []string {
	var out []string
	read := func(sc *bufio.Scanner) {
		sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for sc.Scan() {
			out = append(out, sc.Text())
		}
		if err := sc.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "sort: %v\n", err)
			os.Exit(1)
		}
	}
	if len(files) == 0 {
		read(bufio.NewScanner(os.Stdin))
		return out
	}
	for _, name := range files {
		if name == "-" {
			read(bufio.NewScanner(os.Stdin))
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sort: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		read(sc)
	}
	return out
}

// key возвращает подстроку для сравнения: при column>0 — колонка по табуляции, иначе вся строка; при -b без хвостовых пробелов.
func key(line string, column int, trim bool) string {
	if column > 0 {
		cols := strings.Split(line, "\t")
		if column > len(cols) {
			line = ""
		} else {
			line = cols[column-1]
		}
	}
	if trim {
		line = strings.TrimRight(line, " \t")
	}
	return line
}

// cmp возвращает -1 (a<b), 0 (a==b), 1 (a>b). Приоритет: -M, затем -h, затем -n, иначе строки.
func cmp(a, b string, numeric, month, human, trim bool, col int) int {
	ka := key(a, col, trim)
	kb := key(b, col, trim)
	var v int
	switch {
	case month:
		v = cmpMonth(ka, kb)
	case human:
		v = cmpHuman(ka, kb)
	case numeric:
		v = cmpNum(ka, kb)
	default:
		if ka < kb {
			v = -1
		} else if ka > kb {
			v = 1
		}
	}
	return v
}

func cmpMonth(a, b string) int {
	ma := months[strings.ToLower(strings.TrimSpace(a))]
	mb := months[strings.ToLower(strings.TrimSpace(b))]
	if ma != 0 && mb != 0 {
		if ma < mb {
			return -1
		}
		if ma > mb {
			return 1
		}
		return 0
	}
	if ma != 0 {
		return -1
	}
	if mb != 0 {
		return 1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func cmpHuman(a, b string) int {
	va := parseHumanNumber(a)
	vb := parseHumanNumber(b)
	if va < vb {
		return -1
	}
	if va > vb {
		return 1
	}
	return strings.Compare(a, b)
}

func cmpNum(a, b string) int {
	va := parseNumber(a)
	vb := parseNumber(b)
	if va < vb {
		return -1
	}
	if va > vb {
		return 1
	}
	return strings.Compare(a, b)
}

// sortLines сортирует слайс на месте по заданным флагам.
func sortLines(lines []string, numeric, monthSort, humanSort, reverse, ignoreBlanks bool, column int) {
	sort.Slice(lines, func(i, j int) bool {
		v := cmp(lines[i], lines[j], numeric, monthSort, humanSort, ignoreBlanks, column)
		if reverse {
			v = -v
		}
		return v < 0
	})
}

// sorted возвращает true, если строки в нужном порядке (для -c).
func sorted(lines []string, numeric, monthSort, humanSort, ignoreBlanks bool, column int) bool {
	for i := 1; i < len(lines); i++ {
		if cmp(lines[i-1], lines[i], numeric, monthSort, humanSort, ignoreBlanks, column) > 0 {
			return false
		}
	}
	return true
}

func parseNumber(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// parseHumanNumber парсит 1K, 10MB, 1.5G (основание 1024).
func parseHumanNumber(s string) float64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	for i := len(s) - 1; i >= 0; i-- {
		if m, ok := humanMult[s[i]]; ok {
			if n, err := strconv.ParseFloat(strings.TrimSpace(s[:i]), 64); err == nil {
				return n * m
			}
			break
		}
	}
	n, _ := strconv.ParseFloat(s, 64)
	return n
}

// checkSorted выводит сообщение о порядке (для тестов с перехватом stdout).
func checkSorted(lines []string, numeric, monthSort, humanSort, ignoreBlanks bool, column int) {
	if len(lines) <= 1 {
		fmt.Println("файл отсортирован (пустой или одна строка)")
		return
	}
	if !sorted(lines, numeric, monthSort, humanSort, ignoreBlanks, column) {
		fmt.Println("файл не отсортирован")
		return
	}
	fmt.Println("файл отсортирован")
}

func uniqueLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := []string{lines[0]}
	for i := 1; i < len(lines); i++ {
		if lines[i] != lines[i-1] {
			out = append(out, lines[i])
		}
	}
	return out
}
