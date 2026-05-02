package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/byteme/compiler/analyzer"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/evaluator"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/optimizer"
	"github.com/byteme/compiler/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: byteme <filename.byme>")
		os.Exit(1)
	}

	filename := os.Args[1]
	input, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading file: %s\n", err)
		os.Exit(1)
	}

	// 1. Lexing
	l := lexer.New(string(input))

	// 2. Parsing
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printErrors("Parser", p.Errors())
		os.Exit(1)
	}

	// 3. Semantic Analysis
	env := environment.NewEnvironment()
	a := analyzer.New(env)
	a.Analyze(program)
	if len(a.Errors()) != 0 {
		printErrors("Analyzer", a.Errors())
		os.Exit(1)
	}

	// 4. Optimization
	opt := optimizer.New()
	optimized := opt.Optimize(program).(*ast.Program)

	// 5. Evaluation
	result := evaluator.Eval(optimized, env)
	if result != nil {
		fmt.Println(result.Inspect())
	}
}

func printErrors(phase string, errors []string) {
	fmt.Printf("%s Errors:\n", phase)
	for _, msg := range errors {
		fmt.Printf("\t%s\n", msg)
	}
}
