# ByteMe Language Support

This extension provides syntax highlighting and semantic linting for the ByteMe programming language.

## Features

- **Syntax Highlighting**: Supports all ByteMe keywords, types, and literals.
- **Linting**: Integrates with the ByteMe compiler's `-lint` flag to show errors in the VS Code Problems tab.

## Setup

To use the linter:
1. Ensure the ByteMe compiler is built.
2. Configure a VS Code task to run `go run main.go -lint ${file}` and use the `byteme-linter` problem matcher.

Enjoy coding in ByteMe!
