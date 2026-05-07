package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	file, err := os.Open("object/builtins.go")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	index := 0
	re := regexp.MustCompile(`\{ // (\d+): (\w+)`)
	
	fmt.Println("var builtins = map[string]int{")
	inSlice := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "var Builtins = []*Builtin{") {
			inSlice = true
			continue
		}
		if inSlice && strings.Contains(line, "}") && strings.HasPrefix(strings.TrimSpace(line), "}") {
			// End of entry
		}
		if inSlice && strings.HasPrefix(strings.TrimSpace(line), "},") {
			// Index increments after each entry
		}
		
		matches := re.FindStringSubmatch(line)
		if inSlice && len(matches) == 3 {
			name := matches[2]
			fmt.Printf("\t\"%s\": %d,\n", name, index)
		}
		if inSlice && strings.Contains(line, "Fn: func") {
			index++
		}
	}
	fmt.Println("}")
}
