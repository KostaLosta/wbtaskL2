package main

import "flag"

// Config описывает все поддерживаемые флаги и параметры запуска утилиты.
type Config struct {
	after      int  // -A N: контекст после совпадения
	before     int  // -B N: контекст до совпадения
	context    int  // -C N: симметричный контекст
	count      bool // -c: печатать только количество совпавших строк
	ignoreCase bool // -i: игнорировать регистр
	invert     bool // -v: инвертировать условие совпадения
	fixed      bool // -F: воспринимать шаблон как фиксированную подстроку
	lineNum    bool // -n: печатать номер строки

	pattern string   // сам шаблон
	files   []string // список файлов (если пустой — читаем из stdin)
}

// parseFlags парсит флаги командной строки и заполняет структуру Config.
func parseFlags() Config {
	var cfg Config

	flag.IntVar(&cfg.after, "A", 0, "print N lines of trailing context after matching lines")
	flag.IntVar(&cfg.before, "B", 0, "print N lines of leading context before matching lines")
	flag.IntVar(&cfg.context, "C", 0, "print N lines of output context")
	flag.BoolVar(&cfg.count, "c", false, "print only a count of matching lines per FILE")
	flag.BoolVar(&cfg.ignoreCase, "i", false, "ignore case distinctions")
	flag.BoolVar(&cfg.invert, "v", false, "invert the sense of matching")
	flag.BoolVar(&cfg.fixed, "F", false, "interpret PATTERN as a list of fixed strings")
	flag.BoolVar(&cfg.lineNum, "n", false, "print line number with output lines")

	flag.Parse()
	return cfg
}
