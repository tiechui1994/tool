package main

import (
	"fmt"
	"github.com/tidwall/sjson"
)

func main() {
	data := `{"AA":"CCC"}`
	ans, _ := sjson.SetRaw(data, "AA", "xxxx")
	fmt.Println(ans)
}
