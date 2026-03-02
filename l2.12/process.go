package main

import (
	"bufio"
	"fmt"
	"io"
)

// processStream построчно читает поток r, применяет к строкам функцию match
// и выводит результат с учётом флагов контекста (-A, -B, -C), инверсии (-v),
// подсчёта (-c) и номеров строк (-n).
func processStream(
	name string,
	r io.Reader,
	cfg *Config,
	match func(string) bool,
	multipleFiles bool,
) error {
	scanner := bufio.NewScanner(r)

	type lineInfo struct {
		num  int
		text string
	}

	var prev []lineInfo           // кольцевой буфер строк "до" совпадения
	printed := make(map[int]bool) // уже выведенные номера строк (для устранения дублей)
	lineNum := 0                  // текущий номер строки
	afterRemain := 0              // сколько строк контекста "после" ещё нужно вывести
	matchCount := 0               // счётчик совпавших строк

	for scanner.Scan() {
		lineNum++
		text := scanner.Text()

		isMatch := match(text)
		if cfg.invert {
			isMatch = !isMatch
		}
		if isMatch {
			matchCount++
		}

		if !cfg.count {
			if isMatch {
				// Печатаем накопленный контекст "до".
				for _, p := range prev {
					if !printed[p.num] {
						printLine(name, p.num, p.text, cfg, multipleFiles)
						printed[p.num] = true
					}
				}

				// Печатаем саму совпавшую строку.
				if !printed[lineNum] {
					printLine(name, lineNum, text, cfg, multipleFiles)
					printed[lineNum] = true
				}

				// Запускаем/обновляем счётчик контекста "после".
				if cfg.after > 0 && afterRemain < cfg.after {
					afterRemain = cfg.after
				}
			} else if afterRemain > 0 {
				// Строка не совпала, но попадает в контекст "после".
				if !printed[lineNum] {
					printLine(name, lineNum, text, cfg, multipleFiles)
					printed[lineNum] = true
				}
				afterRemain--
			}
		}

		// Обновляем буфер строк "до" для следующего совпадения.
		if cfg.before > 0 {
			prev = append(prev, lineInfo{num: lineNum, text: text})
			if len(prev) > cfg.before {
				prev = prev[1:]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if cfg.count {
		// Для нескольких файлов — как в обычном grep, имя:кол-во.
		if multipleFiles && name != "" {
			fmt.Printf("%s:%d\n", name, matchCount)
		} else {
			fmt.Printf("%d\n", matchCount)
		}
	}

	return nil
}

// printLine печатает одну строку с учётом флагов -n и множества файлов.
// Формат совместим с выводом стандартного grep.
func printLine(name string, num int, text string, cfg *Config, multipleFiles bool) {
	prefix := ""
	if multipleFiles && name != "" {
		prefix += name + ":"
	}
	if cfg.lineNum {
		prefix += fmt.Sprintf("%d:", num)
	}
	if prefix != "" {
		fmt.Print(prefix)
	}
	fmt.Println(text)
}
