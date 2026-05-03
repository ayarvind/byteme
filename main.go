package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/byteme/compiler/analyzer"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/code"
	"github.com/byteme/compiler/compiler"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/evaluator"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/optimizer"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/object"
	"github.com/byteme/compiler/repl"
	"github.com/byteme/compiler/vm"
)

func init() {
	// Register a native Go function
	evaluator.RegisterNative("goGreet", func(args ...object.Object) object.Object {
		if len(args) != 1 { return object.NULL }
		name := args[0].Inspect()
		return &object.String{Value: fmt.Sprintf("Hello %s, I am a Go function!", name)}
	})
}

func main() {
	compileOnly := flag.Bool("c", false, "compile only, do not run")
	useEvaluator := flag.Bool("eval", false, "use the tree-walk evaluator instead of VM")
	disassemble := flag.Bool("d", false, "disassemble bytecode")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("ByteMe Language Shell v1.0")
		repl.Start(os.Stdin, os.Stdout)
		return
	}

	filename := flag.Args()[0]
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

	if *disassemble {
		comp := compiler.New()
		err := comp.Compile(optimized)
		if err != nil {
			fmt.Printf("Compilation Error: %s\n", err)
			os.Exit(1)
		}
		bytecode := comp.Bytecode()
		fmt.Println("Main Bytecode:")
		fmt.Println(bytecode.Instructions.String())
		
		for i, constant := range bytecode.Constants {
			if fn, ok := constant.(*object.CompiledFunction); ok {
				fmt.Printf("\nConstant Function %d:\n", i)
				fmt.Println(code.Instructions(fn.Instructions).String())
			} else if sl, ok := constant.(*object.StructLiteral); ok {
				for name, meth := range sl.Methods {
					fmt.Printf("\nMethod %s.%s:\n", sl.Name, name)
					fmt.Println(code.Instructions(meth.Instructions).String())
				}
			}
		}
		return
	}

	if *compileOnly {
		comp := compiler.New()
		err := comp.Compile(optimized)
		if err != nil {
			fmt.Printf("Compilation Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Println("Compilation successful.")
		return
	}

	if *useEvaluator {
		result := evaluator.Eval(optimized, env)
		if result != nil && result != object.NULL {
			fmt.Println(result.Inspect())
		}
	} else {
		comp := compiler.New()
		err := comp.Compile(optimized)
		if err != nil {
			fmt.Printf("Compiler Error: %s\n", err)
			return
		}

		machine := vm.New(comp.Bytecode())
		err = machine.Run()
		if err != nil {
			fmt.Printf("VM Error: %s\n", err)
			return
		}

		lastStackElem := machine.LastPoppedStackElem()
		if lastStackElem != nil && lastStackElem != object.NULL {
			fmt.Println(lastStackElem.Inspect())
		}
	}
}

func printErrors(phase string, errors []string) {
	fmt.Printf("%s Errors:\n", phase)
	for _, msg := range errors {
		fmt.Printf("\t%s\n", msg)
	}
}
