package main

import (
	"fmt"
	"net/http"
)

func main() {
	resp, err := http.Get("https://www.tianxibook.com/xiaoshuo/94098167/47269779.html")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(resp.Status)
}
