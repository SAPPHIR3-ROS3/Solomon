package sdk

import "fmt"

func Print(v ...any) {
	fmt.Print(v...)
}

func Println(v ...any) {
	fmt.Println(v...)
}

func Printf(format string, v ...any) {
	fmt.Printf(format, v...)
}
