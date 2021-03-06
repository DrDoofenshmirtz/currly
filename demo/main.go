package main

import (
	"fmt"
	"os"

	"github.com/DrDoofenshmirtz/currly"
)

func main() {
	curl, err := currly.
		Builder().
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
	con := currly.DefaultConnector()
	arg := currly.JSONBodyArg(body)
	sc, res, err := curl(con, arg)

	if err != nil {
		fmt.Println(err)

		os.Exit(42)

		return
	}

	fmt.Println(sc)
	fmt.Println(res)
}
