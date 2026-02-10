package main

import (
	"fmt"
	"os"
)

func Foo() error {
	var err *os.PathError = nil
	return err
}

func main() {
	err := Foo()
	fmt.Println(err)
	fmt.Println(err == nil)
}

// Программа выведет <nil> и false. Связано с тем, что под капотом интерфейсы содержат в себе два элемента, а именно тип и значение. Интерфейс равен nil, только если оба элемента равны nil.
// В данной программе значение равно nil, но интерфейс имеет тип *os.PathError.
