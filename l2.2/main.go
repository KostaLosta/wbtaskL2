package main

import "fmt"

func test() (x int) {
	defer func() {
		x++
	}()
	x = 1
	return
}

func anotherTest() int {
	var x int
	defer func() {
		x++
	}()
	x = 1
	return x
}

func main() {
	fmt.Println(test())
	fmt.Println(anotherTest())
}

// Программа выведет два значения: 2 и 1. Разница результатов обусловлена тем, что в функции test() указан возвращаемый параметр, а в функции anotherTest() нет.
// Согласно документации, если возвращаемый параметр указан, то defer может менять результат, в противном случае результат не меняется.
