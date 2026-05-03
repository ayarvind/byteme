package object

// FunctionRunner is set by the VM at startup so that built-ins (like the HTTP
// module) can call back into ByteMe user-defined compiled functions without
// creating a hard import cycle between object ↔ vm.
//
// Signature: RunFunction(fn, constants, globals, args) → result Object
var RunFunction func(fn *CompiledFunction, constants []Object, globals []Object, args []Object) Object
