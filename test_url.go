package main
import (
	"fmt"
	"net/url"
)
func main() {
	u, _ := url.Parse("https://[::1]:8080/foo")
	fmt.Printf("Host: %q, Hostname: %q, Port: %q\n", u.Host, u.Hostname(), u.Port())
}
