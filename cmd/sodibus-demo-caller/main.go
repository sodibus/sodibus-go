package main

import "log"
import "time"
import "strconv"
import "math/rand"
import "github.com/sodibus/sodigo"

func main() {
	rand.Seed(time.Now().UnixNano())

	var r string
	var err error

	c := sodigo.NewCaller("127.0.0.1:7788")

	i := 0

	var t uint64

	for {
		v1 := strconv.Itoa(rand.Intn(1000))
		v2 := strconv.Itoa(rand.Intn(1000))
		now := time.Now().UnixNano()
		r, err = c.Invoke("calculator", "multiply", []string{v1, v2})
		if err != nil {
			log.Println("Error:", err)
			return
		}

		dt := time.Now().UnixNano() - now

		log.Printf("%d Calculate Result: %s * %s = %s\n", dt, v1, v2, r)

		t = t + uint64(dt)
		i = i + 1

		if i > 10000 {
			break
		}
	}

	log.Println("Avg dt", t/uint64(i), "ns")
}
