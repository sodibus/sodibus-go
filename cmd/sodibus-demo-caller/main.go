package main

import "log"
import "github.com/sodibus/sodigo"

func main() {
	var r string
	c, err := sodigo.DialAsCaller("127.0.0.1:7788")
	if err != nil { 
		log.Println("Error:", err)
		return 
	}
	r, err = c.Invoke("calculator", "multiply", []string { "2", "4" })
	if err != nil { 
		log.Println("Error:", err)
		return 
	}
	log.Printf("Calculate Result: %s * %s = %s\n", "2", "4", r)
}
