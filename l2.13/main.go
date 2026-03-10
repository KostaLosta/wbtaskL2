package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

type interval struct {
	start int
	end   int
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(args []string, in io.Reader, out io.Writer, errOut io.Writer) error {

	fs := flag.NewFlagSet("cut", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		fieldsSpec string
		delSpec    string
		onlySep    bool
	)
	fs.StringVar(&fieldsSpec, "f", "", "номера полей (например: 1,3-5)")
	fs.StringVar(&delSpec, "d", "\t", "разделитель (один символ)")
	fs.BoolVar(&onlySep, "s", false, "только строки, содержащие разделитель")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(fieldsSpec) == "" {
		_, _ = fmt.Fprintln(errOut, "ошибка: флаг -f обязателен")
		fs.Usage()
		return errors.New("missing -f")
	}

	delimiter, delimiterByte, isSingleByte, err := parseDelimiter(delSpec)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, "ошибка:", err)
		fs.Usage()
		return err
	}

	intervals, err := parseFieldsSpec(fieldsSpec)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, "ошибка:", err)
		fs.Usage()
		return err
	}

	sc := bufio.NewScanner(in)

	const maxLine = 10 * 1024 * 1024
	sc.Buffer(make([]byte, 64*1024), maxLine)

	bw := bufio.NewWriter(out)
	defer bw.Flush()

	for sc.Scan() {
		line := sc.Text()
		if onlySep && !containsDelimiter(line, delimiter, delimiterByte, isSingleByte) {
			continue
		}
		fields := selectFields(line, delimiter, delimiterByte, isSingleByte, intervals)
		_, _ = bw.WriteString(strings.Join(fields, delimiter))
		_ = bw.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		_, _ = fmt.Fprintln(errOut, "ошибка чтения:", err)
		return err
	}
	return nil
}

func parseDelimiter(s string) (delim string, delimByte byte, isSingleByte bool, err error) {
	if s == "" {
		return "", 0, false, errors.New("пустой разделитель недопустим")
	}
	runes := []rune(s)
	if len(runes) != 1 {
		return "", 0, false, fmt.Errorf("разделитель должен быть одним символом, получено: %q", s)
	}
	delim = string(runes[0])
	b := []byte(delim)
	isSingleByte = len(b) == 1
	if isSingleByte {
		delimByte = b[0]
	}
	return delim, delimByte, isSingleByte, nil
}

func parseFieldsSpec(spec string) ([]interval, error) {
	parts := strings.Split(spec, ",")
	out := make([]interval, 0, len(parts))
	for _, raw := range parts {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			return nil, fmt.Errorf("пустой токен в -f: %q", spec)
		}
		if strings.Contains(tok, "-") {
			if strings.Count(tok, "-") != 1 {
				return nil, fmt.Errorf("некорректный диапазон: %q", tok)
			}
			a, b, ok := strings.Cut(tok, "-")
			if !ok {
				return nil, fmt.Errorf("некорректный диапазон: %q", tok)
			}
			start, err := strconv.Atoi(strings.TrimSpace(a))
			if err != nil || start <= 0 {
				return nil, fmt.Errorf("некорректное начало диапазона: %q", tok)
			}
			end, err := strconv.Atoi(strings.TrimSpace(b))
			if err != nil || end <= 0 {
				return nil, fmt.Errorf("некорректный конец диапазона: %q", tok)
			}
			if start > end {
				return nil, fmt.Errorf("начало диапазона больше конца: %q", tok)
			}
			out = append(out, interval{start: start, end: end})
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("некорректный номер поля: %q", tok)
		}
		out = append(out, interval{start: n, end: n})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].start != out[j].start {
			return out[i].start < out[j].start
		}
		return out[i].end < out[j].end
	})

	merged := out[:0]
	for _, it := range out {
		if len(merged) == 0 {
			merged = append(merged, it)
			continue
		}
		last := &merged[len(merged)-1]
		if it.start <= last.end+1 {
			if it.end > last.end {
				last.end = it.end
			}
			continue
		}
		merged = append(merged, it)
	}
	return merged, nil
}

func containsDelimiter(line, delim string, delimByte byte, isSingleByte bool) bool {
	if isSingleByte {
		return strings.IndexByte(line, delimByte) >= 0
	}
	return strings.Contains(line, delim)
}

func selectFields(line, delim string, delimByte byte, isSingleByte bool, wanted []interval) []string {
	if len(wanted) == 0 {
		return nil
	}

	res := make([]string, 0, 8)
	fieldIdx := 0
	wi := 0

	needField := func(i int) bool {
		for wi < len(wanted) && i > wanted[wi].end {
			wi++
		}
		return wi < len(wanted) && i >= wanted[wi].start && i <= wanted[wi].end
	}

	if isSingleByte {
		d := delimByte
		start := 0
		for i := 0; i <= len(line); i++ {
			if i == len(line) || line[i] == d {
				fieldIdx++
				if needField(fieldIdx) {
					res = append(res, line[start:i])
				}
				start = i + 1
				if wi >= len(wanted) {
					break
				}
			}
		}
		return res
	}

	start := 0
	pos := 0
	dlen := len(delim)
	for {
		if pos > len(line) {
			break
		}
		idx := strings.Index(line[pos:], delim)
		var end, next int
		if idx < 0 {
			end = len(line)
			next = len(line) + 1
		} else {
			end = pos + idx
			next = end + dlen
		}

		fieldIdx++
		if needField(fieldIdx) {
			res = append(res, line[start:end])
		}
		start = next
		pos = next
		if idx < 0 || wi >= len(wanted) {
			break
		}
	}
	return res
}
