package main

import (
	"sort"
	"strings"
)

func sortRunes(s string) string {
	r := []rune(s)
	sort.Slice(r, func(i, j int) bool { return r[i] < r[j] })
	return string(r)
}

func Anagrams(words []string) map[string][]string {
	firstSeen := make(map[string]string)
	groups := make(map[string]map[string]struct{})

	for _, w := range words {
		lower := strings.ToLower(w)
		if lower == "" {
			continue
		}
		canonical := sortRunes(lower)
		if _, ok := firstSeen[canonical]; !ok {
			firstSeen[canonical] = lower
		}
		if groups[canonical] == nil {
			groups[canonical] = make(map[string]struct{})
		}
		groups[canonical][lower] = struct{}{}
	}

	result := make(map[string][]string)
	for canonical, set := range groups {
		if len(set) <= 1 {
			continue
		}
		keyWord := firstSeen[canonical]
		slice := make([]string, 0, len(set))
		for word := range set {
			slice = append(slice, word)
		}
		sort.Strings(slice)
		result[keyWord] = slice
	}
	return result
}

func main() {

	input := []string{"пятак", "пятка", "тяпка", "листок", "слиток", "столик", "стол"}
	output := Anagrams(input)
	for k, v := range output {
		println(k+":", strings.Join(v, ", "))
	}
}
