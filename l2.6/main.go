package main

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func test() *customError {
	// ... do something
	return nil
}

func main() {
	var err error
	err = test()
	if err != nil {
		println("error")
		return
	}
	println("ok")
}

// Программа выведет error.  Интерфейс хранит в себе тип и значение. В мейне создается переменная err, которая изначально считается nil, т.к. при объявлении у нее тип значение равны nil.
// Далее функция test() возвращает nil значение, но у же с типом *customError.
// Дальше проводится проверка на nil, но наша переменная уже типизированная, с типом customError и значением nil.
// ИСходя из этого, выводится "error", а не "ок".
