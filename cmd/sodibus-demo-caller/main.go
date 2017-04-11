package main

import "fmt"
import "github.com/sodibus/sodigo"

func main() {
	var r string
	c, err := sodigo.DialAsCaller("127.0.0.1:7788")
	if err != nil { return }
	r, err = c.Invoke("calculator", "multiply", []string { "2", "4" })
	if err != nil { return }
	fmt.Printf("Calculate Result: %s * %s = %s", "2", "4", r)
}
