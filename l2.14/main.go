package main

import (
	"fmt"
	"time"
)

// or содержит основную логику объединения done-каналов. Возвращаемый канал закрывается, как только закроется любой из исходных.
func or(channels ...<-chan interface{}) <-chan interface{} {
	switch len(channels) {
	case 0:
		// Нет ни одного канала — возвращаем nil, ожидание на нём будет вечным.
		return nil
	case 1:
		// Один канал — нет смысла оборачивать его ещё раз.
		return channels[0]
	}

	out := make(chan interface{})

	go func() {
		defer close(out)

		switch len(channels) {
		case 2:
			select {
			case <-channels[0]:
				return
			case <-channels[1]:
				return
			}
		default:
			mid := len(channels) / 2
			select {
			case <-or(channels[:mid]...):
				return
			case <-or(channels[mid:]...):
				return
			}
		}
	}()

	return out
}

func main() {
	sig := func(after time.Duration) <-chan interface{} {
		c := make(chan interface{})
		go func() {
			defer close(c)
			time.Sleep(after)
		}()
		return c
	}

	start := time.Now()
	<-or(
		sig(2*time.Hour),
		sig(5*time.Minute),
		sig(1*time.Second),
		sig(1*time.Hour),
		sig(1*time.Minute),
	)
	fmt.Printf("done after %v\n", time.Since(start))
}
