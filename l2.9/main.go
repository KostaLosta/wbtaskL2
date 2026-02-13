/*
Написать функцию Go, осуществляющую примитивную распаковку строки, содержащей повторяющиеся символы/руны.

Примеры работы функции:

Вход: "a4bc2d5e"
Выход: "aaaabccddddde"

Вход: "abcd"
Выход: "abcd" (нет цифр — ничего не меняется)

Вход: "45"
Выход: "" (некорректная строка, т.к. в строке только цифры — функция должна вернуть ошибку)

Вход: ""
Выход: "" (пустая строка -> пустая строка)

Дополнительное задание
Поддерживать escape-последовательности вида \:

Вход: "qwe\4\5"
Выход: "qwe45" (4 и 5 не трактуются как числа, т.к. экранированы)

Вход: "qwe\45"
Выход: "qwe44444" (\4 экранирует 4, поэтому распаковывается только 5)

Требования к реализации
Функция должна корректно обрабатывать ошибочные случаи (возвращать ошибку, например, через error), и проходить unit-тесты.

Код должен быть статически анализируем (vet, golint).
*/

package main

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidString = errors.New("invalid string")

func Unpack(s string) (string, error) {
	if s == "" {
		return "", nil
	}

	runes := []rune(s)

	if runes[0] >= '0' && runes[0] <= '9' {
		return "", ErrInvalidString
	}

	var lastRune rune
	var result strings.Builder

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' {
			i++
			if i >= len(runes) {
				return "", ErrInvalidString
			}
			r = runes[i]
			result.WriteRune(r)
			lastRune = r
			continue
		}
		if r >= '0' && r <= '9' {
			for k := 0; k < int(r-'0')-1; k++ {
				result.WriteRune(lastRune)
			}
			continue
		}
		result.WriteRune(r)
		lastRune = r
	}
	return result.String(), nil

}

func main() {
	out, err := Unpack(`qwe\4\5`)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(out)
}
