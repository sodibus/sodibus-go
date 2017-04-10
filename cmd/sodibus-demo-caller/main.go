package main

import "fmt"
import "github.com/sodibus/sodigo"

func main() {
	client := sodigo.NewClient(":7788")
	fmt.Println(client.Invoke("multiply", map[string]string{ "src" : "10", "dst":"10" }))
}
