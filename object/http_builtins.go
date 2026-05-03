package object

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// vmConstants and vmGlobals are live pointer-to-slices set by the VM so that
// HTTP handlers always see the fully-populated runtime state even though
// SetVMContext is called before the script finishes compiling.
var (
	vmConstantsPtr *[]Object
	vmGlobalsPtr   *[]Object
)

// SetVMContext stores pointers to the VM's constants and globals arrays.
// Using pointers ensures HTTP handlers see the live state at request time.
func SetVMContext(constants *[]Object, globals *[]Object) {
	vmConstantsPtr = constants
	vmGlobalsPtr   = globals
}

// HTTPBuiltins are the web/networking built-ins exposed to ByteMe scripts.
// They are appended to the main Builtins slice at init time.

// httpRoutes stores registered routes: path → CompiledFunction
type httpRouteEntry struct {
	fn      *CompiledFunction
	constants []Object
	globals   []Object
}

var (
	httpRoutes   = make(map[string]*httpRouteEntry)
	httpRoutesMu sync.RWMutex
)

// RegisterHTTPBuiltins appends all HTTP built-ins to Builtins and returns the
// starting index of the first HTTP built-in so the compiler can map names → indices.
func RegisterHTTPBuiltins() int {
	start := len(Builtins)

	// Index start+0: httpHandle(path, fn)
	Builtins = append(Builtins, &Builtin{Fn: builtinHTTPHandle})

	// Index start+1: httpServe(port)
	Builtins = append(Builtins, &Builtin{Fn: builtinHTTPServe})

	// Index start+2: httpGet(url)
	Builtins = append(Builtins, &Builtin{Fn: builtinHTTPGet})

	// Index start+3: httpPost(url, body)
	Builtins = append(Builtins, &Builtin{Fn: builtinHTTPPost})

	// Index start+4: httpResponse(status, body)
	Builtins = append(Builtins, &Builtin{Fn: builtinHTTPResponse})

	return start
}

func builtinHTTPHandle(args ...Object) Object {
	if len(args) != 2 {
		return &Error{Message: "httpHandle requires 2 arguments: (path, handler)"}
	}
	path, ok := args[0].(*String)
	if !ok {
		return &Error{Message: "httpHandle: first argument must be a string path"}
	}
	fn, ok := args[1].(*CompiledFunction)
	if !ok {
		return &Error{Message: "httpHandle: second argument must be a function"}
	}

	httpRoutesMu.Lock()
	httpRoutes[path.Value] = &httpRouteEntry{fn: fn}
	httpRoutesMu.Unlock()

	fmt.Printf("[ByteMe HTTP] Registered route: %s\n", path.Value)
	return NULL
}

// builtinHTTPServe starts the HTTP server.
// httpServe(port: int)  — blocks until the process exits.
func builtinHTTPServe(args ...Object) Object {
	port := 8080
	if len(args) == 1 {
		if p, ok := args[0].(*Integer); ok {
			port = int(p.Value)
		}
	}

	mux := http.NewServeMux()

	// Register a catch-all handler that dispatches to ByteMe route handlers.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpRoutesMu.RLock()
		entry, ok := httpRoutes[r.URL.Path]
		httpRoutesMu.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		// Build a ByteMe request map
		bodyBytes, _ := io.ReadAll(r.Body)
		reqMap := NewStringMap(map[string]Object{
			"method": &String{Value: r.Method},
			"path":   &String{Value: r.URL.Path},
			"query":  &String{Value: r.URL.RawQuery},
			"body":   &String{Value: string(bodyBytes)},
		})

		// Build headers map
		headers := NewStringMap(make(map[string]Object))
		for k, v := range r.Header {
			headers.Pairs[k] = MapPair{Key: &String{Value: k}, Value: &String{Value: strings.Join(v, ", ")}}
		}
		reqMap.Pairs["headers"] = MapPair{Key: &String{Value: "headers"}, Value: headers}

		// Call the ByteMe handler — always use the live constants/globals via pointers
		var result Object = NULL
		fmt.Printf("[ByteMe HTTP] Dispatching to handler for: %s\n", r.URL.Path)
		if RunFunction != nil && vmConstantsPtr != nil && vmGlobalsPtr != nil {
			result = RunFunction(entry.fn, *vmConstantsPtr, *vmGlobalsPtr, []Object{reqMap})
		} else {
			fmt.Printf("[ByteMe HTTP] ERROR: Runner not initialised (RunFunction: %v, constants: %v, globals: %v)\n", 
				RunFunction != nil, vmConstantsPtr != nil, vmGlobalsPtr != nil)
			result = &Error{Message: "VM runner not initialised"}
		}
		fmt.Printf("[ByteMe HTTP] Handler returned for: %s\n", r.URL.Path)

		// Interpret result: expect a Map with "status" and "body"
		status := 200
		body := ""
		contentType := "text/plain"

		if resMap, ok := result.(*Map); ok {
			if sObj, ok := resMap.Get("status"); ok {
				if s, ok := sObj.(*Integer); ok {
					status = int(s.Value)
				}
			}
			if bObj, ok := resMap.Get("body"); ok {
				if b, ok := bObj.(*String); ok {
					body = b.Value
				}
			}
			if ctObj, ok := resMap.Get("contentType"); ok {
				if ct, ok := ctObj.(*String); ok {
					contentType = ct.Value
				}
			}
		} else if s, ok := result.(*String); ok {
			body = s.Value
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("[ByteMe HTTP] Server listening on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		return &Error{Message: "httpServe: " + err.Error()}
	}
	return NULL
}

// builtinHTTPGet performs an HTTP GET.
// httpGet(url: string) -> map{status, body, headers}
func builtinHTTPGet(args ...Object) Object {
	if len(args) < 1 {
		return &Error{Message: "httpGet requires a URL argument"}
	}
	urlStr, ok := args[0].(*String)
	if !ok {
		return &Error{Message: "httpGet: first argument must be a string URL"}
	}

	resp, err := http.Get(urlStr.Value) //nolint:gosec
	if err != nil {
		return &Error{Message: "httpGet: " + err.Error()}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	headers := NewStringMap(make(map[string]Object))
	for k, v := range resp.Header {
		headers.Pairs[k] = MapPair{Key: &String{Value: k}, Value: &String{Value: strings.Join(v, ", ")}}
	}

	return NewStringMap(map[string]Object{
		"status":  &Integer{Value: int64(resp.StatusCode)},
		"body":    &String{Value: string(bodyBytes)},
		"headers": headers,
	})
}

// builtinHTTPPost performs an HTTP POST.
// httpPost(url: string, body: string) -> map{status, body}
func builtinHTTPPost(args ...Object) Object {
	if len(args) < 2 {
		return &Error{Message: "httpPost requires (url, body) arguments"}
	}
	urlStr, ok1 := args[0].(*String)
	bodyStr, ok2 := args[1].(*String)
	if !ok1 || !ok2 {
		return &Error{Message: "httpPost: both arguments must be strings"}
	}

	contentType := "application/json"
	resp, err := http.Post(urlStr.Value, contentType, strings.NewReader(bodyStr.Value)) //nolint:gosec
	if err != nil {
		return &Error{Message: "httpPost: " + err.Error()}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	return NewStringMap(map[string]Object{
		"status": &Integer{Value: int64(resp.StatusCode)},
		"body":   &String{Value: string(bodyBytes)},
	})
}

// builtinHTTPResponse creates a response map.
// httpResponse(status: int, body: string) -> map
// httpResponse(status: int, body: string, contentType: string) -> map
func builtinHTTPResponse(args ...Object) Object {
	if len(args) < 2 {
		return &Error{Message: "httpResponse requires (status, body) arguments"}
	}
	status, ok1 := args[0].(*Integer)
	body, ok2 := args[1].(*String)
	if !ok1 || !ok2 {
		return &Error{Message: "httpResponse: status must be int, body must be string"}
	}

	res := NewStringMap(map[string]Object{
		"status": status,
		"body":   body,
	})
	if len(args) == 3 {
		if ct, ok := args[2].(*String); ok {
			res.Pairs["contentType"] = MapPair{Key: &String{Value: "contentType"}, Value: ct}
		}
	}
	return res
}
