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
	"github.com/byteme/compiler/token"
	"github.com/byteme/compiler/vm"
	"math/rand"
	"strings"
	"time"
)

var keywordHelp = map[string]string{
	"let":       "Keyword: let - Declares a block-scoped variable.",
	"const":     "Keyword: const - Declares a block-scoped constant.",
	"fn":        "Keyword: fn - Declares a function.",
	"if":        "Keyword: if - Conditional execution.",
	"else":      "Keyword: else - Alternative conditional branch.",
	"while":     "Keyword: while - Loop while condition is true.",
	"for":       "Keyword: for - Loop construct.",
	"in":        "Keyword: in - Used in for-each loops.",
	"return":    "Keyword: return - Exit function and return a value.",
	"struct":    "Keyword: struct - Defines a structured type.",
	"enum":      "Keyword: enum - Defines an enumeration.",
	"interface": "Keyword: interface - Defines a contract.",
	"import":    "Keyword: import - Includes another module.",
	"from":      "Keyword: from - Used in specific imports.",
	"as":        "Keyword: as - Aliasing in imports.",
	"spawn":     "Keyword: spawn - Concurrency primitive.",
	"await":     "Keyword: await - Wait for async operation.",
	"async":     "Keyword: async - Declares an asynchronous function.",
	"try":       "Keyword: try - Error handling block.",
	"catch":     "Keyword: catch - Handle thrown errors.",
	"finally":   "Keyword: finally - Cleanup block.",
	"throw":     "Keyword: throw - Raise an error.",
	"yield":     "Keyword: yield - Yield a value in a generator.",
	"int":       "Type: int - 64-bit integer.",
	"float":     "Type: float - 64-bit floating point number.",
	"string":    "Type: string - UTF-8 encoded string.",
	"bool":      "Type: bool - Boolean value (true or false).",
	"char":      "Type: char - Single Unicode character.",
	"any":       "Type: any - Dynamic type that can hold any value.",
	"void":      "Type: void - Represents the absence of a value.",
}

func init() {
	rand.Seed(time.Now().UnixNano())
	
	// Register a native Go function
	evaluator.RegisterNative("goGreet", func(args ...object.Object) object.Object {
		if len(args) != 1 { return object.NULL }
		name := args[0].Inspect()
		return &object.String{Value: fmt.Sprintf("Hello %s, I am a Go function!", name)}
	})

	evaluator.RegisterNative("mathRand", func(args ...object.Object) object.Object {
		if len(args) == 2 {
			min, ok1 := args[0].(*object.Integer)
			max, ok2 := args[1].(*object.Integer)
			if ok1 && ok2 {
				return &object.Integer{Value: min.Value + rand.Int63n(max.Value-min.Value)}
			}
		}
		return &object.Integer{Value: rand.Int63()}
	})
}

func main() {
	compileOnly := flag.Bool("c", false, "compile only, do not run")
	useEvaluator := flag.Bool("eval", false, "use the tree-walk evaluator instead of VM")
	disassemble := flag.Bool("d", false, "disassemble bytecode")
	lintOnly := flag.Bool("lint", false, "lint only (lexer, parser, and semantic analysis)")
	hoverPos := flag.String("hover", "", "get hover info at file:line:col")
	defPos := flag.String("definition", "", "get definition at file:line:col")
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
	a := analyzer.New(env, string(input), filename)
	a.Analyze(program)
	
	// If we are just linting, we exit on errors.
	// But for hover/definition, we want to provide info even if the program has errors elsewhere.
	if *lintOnly && len(a.Errors()) != 0 {
		printErrors("Analyzer", a.Errors())
		os.Exit(1)
	}

	if *hoverPos != "" {
		targetFile, targetLine, targetCol, err := parsePos(*hoverPos)
		if err != nil {
			fmt.Printf("Invalid hover position format: %s\n", err)
			os.Exit(1)
		}

		var bestSym *environment.Symbol
		for key, sym := range a.ResolvedSymbols {
			kFile, kLine, kCol, _ := parsePos(key)
			if kFile == targetFile && kLine == targetLine {
				if targetCol >= kCol && targetCol < kCol+len(sym.Name) {
					bestSym = &sym
					break
				}
			}
		}

		if bestSym != nil {
			fmt.Printf("Type: %s\n", bestSym.Type)
			
			// Resolve struct info for instances
			infoSym := bestSym
			if bestSym.Type != "struct" && bestSym.Type != "function" && bestSym.Type != "builtin" && 
			   bestSym.Type != "int" && bestSym.Type != "float" && bestSym.Type != "string" && 
			   bestSym.Type != "bool" && bestSym.Type != "char" && bestSym.Type != "array" && 
			   bestSym.Type != "map" && bestSym.Type != "any" && bestSym.Type != "void" {
				for _, s := range a.ResolvedSymbols {
					if s.Name == bestSym.Type && s.Type == "struct" {
						infoSym = &s
						break
					}
				}
			}

			if infoSym.Type == "function" || infoSym.Type == "builtin" {
				params := strings.Join(infoSym.Params, ", ")
				fmt.Printf("Signature: fn(%s) -> %s\n", params, infoSym.ReturnType)
			}
			if infoSym.Type == "struct" {
				fmt.Printf("Fields (%d):\n", len(infoSym.Fields))
				for _, f := range infoSym.Fields {
					fmt.Printf("  - %s\n", f)
				}
				fmt.Printf("Methods (%d):\n", len(infoSym.Methods))
				for _, m := range infoSym.Methods {
					fmt.Printf("  - %s\n", m)
				}
			}
			if bestSym.Docstring != "" {
				fmt.Printf("\n%s\n", bestSym.Docstring)
			}
			if bestSym.IsConst {
				fmt.Println("Constant")
			}
		} else {
			// Check for keywords or other tokens
			tokLit, ok := findToken(string(input), targetLine, targetCol)
			if ok {
				if help, ok := keywordHelp[tokLit]; ok {
					fmt.Println(help)
				} else {
					fmt.Println("No information found at this position.")
				}
			} else {
				fmt.Println("No information found at this position.")
			}
		}
		return
	}

	if *defPos != "" {
		targetFile, targetLine, targetCol, err := parsePos(*defPos)
		if err != nil {
			fmt.Printf("Invalid definition position format: %s\n", err)
			os.Exit(1)
		}

		var bestSym *environment.Symbol
		for key, sym := range a.ResolvedSymbols {
			kFile, kLine, kCol, _ := parsePos(key)
			if kFile == targetFile && kLine == targetLine {
				if targetCol >= kCol && targetCol < kCol+len(sym.Name) {
					bestSym = &sym
					break
				}
			}
		}

		if bestSym != nil {
			if bestSym.Filename != "" {
				fmt.Printf("Definition: %s:%d:%d\n", bestSym.Filename, bestSym.Line, bestSym.Column)
			} else {
				fmt.Println("Built-in symbol.")
			}
		} else {
			fmt.Println("No definition found at this position.")
		}
		return
	}

	if len(a.Errors()) != 0 {
		printErrors("Analyzer", a.Errors())
		os.Exit(1)
	}

	// 4. Optimization
	opt := optimizer.New()
	optimized := opt.Optimize(program).(*ast.Program)

	if *disassemble {
		comp := compiler.New()
		comp.Filename = filename
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
		comp.Filename = filename
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
		comp.Filename = filename
		err := comp.Compile(optimized)
		if err != nil {
			fmt.Printf("Compiler Error: %s\n", err)
			return
		}

		machine := vm.New(comp.Bytecode())
		err = machine.Run()
		if err != nil {
			fmt.Printf("VM Error: %s\n", err)
			fmt.Println(machine.Trace())
			return
		}

		lastStackElem := machine.LastPoppedStackElem()
		if lastStackElem != nil && lastStackElem != object.NULL {
			fmt.Println(lastStackElem.Inspect())
		}
	}
}

func parsePos(pos string) (string, int, int, error) {
	lastColon := strings.LastIndex(pos, ":")
	if lastColon == -1 {
		return "", 0, 0, fmt.Errorf("missing last colon")
	}
	secondLastColon := strings.LastIndex(pos[:lastColon], ":")
	if secondLastColon == -1 {
		return "", 0, 0, fmt.Errorf("missing second last colon")
	}

	file := pos[:secondLastColon]
	// Normalize path for Windows consistency
	file = strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	
	lineStr := pos[secondLastColon+1 : lastColon]
	colStr := pos[lastColon+1:]

	var line, col int
	fmt.Sscanf(lineStr, "%d", &line)
	fmt.Sscanf(colStr, "%d", &col)

	return file, line, col, nil
}

func findToken(source string, line, col int) (string, bool) {
	l := lexer.New(source)
	for {
		tok := l.NextToken()
		if tok.Type == token.EOF {
			break
		}
		if tok.Line == line && col >= tok.Column && col < tok.Column+len(tok.Literal) {
			return tok.Literal, true
		}
		if tok.Line > line {
			break
		}
	}
	return "", false
}

func printErrors(phase string, errors []string) {
	fmt.Printf("%s Errors:\n", phase)
	for _, msg := range errors {
		fmt.Printf("\t%s\n", msg)
	}
}
