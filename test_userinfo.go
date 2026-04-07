package main
import (
	"fmt"
	"net/url"
)
func main() {
	u := url.UserPassword("x-access-token", "my pass+word")
	fmt.Println(u.String())
}
