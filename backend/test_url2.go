package main

import (
	"fmt"
	"net/url"
)

func main() {
	u, _ := url.Parse("https://[::1]")
	fmt.Printf("u.Host for https://[::1] is %q\n", u.Host)

	u2, _ := url.Parse("https://[::1]:8080")
	fmt.Printf("u.Host for https://[::1]:8080 is %q\n", u2.Host)
}
