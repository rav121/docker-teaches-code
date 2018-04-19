package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// Read text from stdin, count the number of words and write the result
// to stdout.
func main() {
	buffer, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	strings := strings.Fields(string(buffer))
	fmt.Println(len(strings))
}
