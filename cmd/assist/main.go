package main

import "fmt"

func main() {
	fmt.Println("dblocker assist ready")
	select {} // block forever
}
