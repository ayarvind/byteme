package repl

import (
	"bufio"
	"fmt"
	"io"

	"github.com/byteme/compiler/analyzer"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/evaluator"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/parser"
)

const PROMPT = "byteme >> "

func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	env := environment.NewEnvironment()

	for {
		fmt.Fprint(out, PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		if line == "exit" || line == "quit" {
			return
		}

		l := lexer.New(line)
		p := parser.New(l)
		program := p.ParseProgram()

		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		// Analyze
		a := analyzer.New(env, line, "repl")
		a.Analyze(program)
		if len(a.Errors()) != 0 {
			printParserErrors(out, a.Errors())
			continue
		}

		result := evaluator.Eval(program, env)
		if result != nil {
			fmt.Fprintf(out, "%s\n", result.Inspect())
		}
	}
}

func printParserErrors(out io.Writer, errors []string) {
	fmt.Fprintln(out, "Errors:")
	for _, msg := range errors {
		fmt.Fprintf(out, "\t%s\n", msg)
	}
}
