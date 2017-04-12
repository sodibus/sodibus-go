package main

import "log"
import "time"
import "strconv"
import "math/rand"
import "github.com/sodibus/sodigo"

func main() {
	rand.Seed(time.Now().UnixNano())

	var r string
	c, err := sodigo.DialAsCaller("127.0.0.1:7788")
	if err != nil { 
		log.Println("Error:", err)
		return 
	}

	i := 0

	for {
		v1  := strconv.Itoa(rand.Intn(1000))
		v2  := strconv.Itoa(rand.Intn(1000))
		now := time.Now().UnixNano()
		r, err = c.Invoke("calculator", "multiply", []string { v1, v2 })
		if err != nil {
			log.Println("Error:", err)
			return
		}
		log.Printf("%d Calculate Result: %s * %s = %s\n", time.Now().UnixNano() - now, "2", "4", r)

		i = i + 1

		if i > 10000 { break }
	}
}
