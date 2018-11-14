package main

import (
	"fmt"
	"os"

	"github.com/DrDoofenshmirtz/currly"
)

func main() {
	curl, err := currly.
		Builder(currly.DefaultConnector()).
		POST().
		HTTPS().
		Host("jsonplaceholder.typicode.com").
		PathSegment("posts").
		Build()

	if err != nil {
		fmt.Println(err)

		os.Exit(42)

		return
	}

	body := map[string]interface{}{
		"title":  "Hi currly!",
		"body":   "Hello, currly.",
		"userId": 42,
	}
	arg := currly.JSONBodyArg(body)
	status, res, err := curl(arg)

	if err != nil {
		fmt.Println(err)

		os.Exit(42)

		return
	}

	fmt.Println(status)
	fmt.Println(res)
}
