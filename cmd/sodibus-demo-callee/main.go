package main

import "strconv"
import "github.com/sodibus/sodigo"

func main() {
	c, err := sodigo.DialAsCallee("127.0.0.1:7788", []string { "calculator" })
	c.Handler = func(service string, method string, arguments []string) string {
		switch method {
			case "multiply": {
				var x, y int
				x, _ = strconv.Atoi(arguments[0])
				y, _ = strconv.Atoi(arguments[1])
				return strconv.Itoa(x * y)
			}
		}
		return "UNKNOWN"
	}
	if err != nil { return }
}

