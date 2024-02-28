package main

import "fmt"

type DataInterface[T any] interface {
	Size() int
	Add__0(v ...T)
	Add__1(v DataInterface[T])
	IndexOf__0(v T) int
	IndexOf__1(pos int, v T) int
}

func Demo__0() { //@Demo__0
}

func Demo__1(n int) int { //@Demo__1
	return n * 100
}

func Demo__2(n1 int, n2 int) { //@Demo__2
	fmt.Println(n1, n2)
}

func Demo__3(s ...string) { //@Demo__3
}

type N struct {
}

func (m *N) Add__0(a int) { //@Add__0
}

func (m *N) Add__1(a ...int) { //@Add__1
}
