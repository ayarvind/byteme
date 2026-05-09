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
	closure *Closure
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

	// Index start+5: httpDo(method, url, body, headers)
	Builtins = append(Builtins, &Builtin{Fn: builtinHTTPDo})

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
	
	var closure *Closure
	switch arg := args[1].(type) {
	case *CompiledFunction:
		closure = &Closure{Fn: arg}
	case *Closure:
		closure = arg
	default:
		return &Error{Message: fmt.Sprintf("httpHandle: second argument must be a function, got %T", args[1])}
	}

	httpRoutesMu.Lock()
	httpRoutes[path.Value] = &httpRouteEntry{closure: closure}
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
		fmt.Printf("[ByteMe HTTP] Request Path: '%s'\n", r.URL.Path)
		entry, ok := httpRoutes[r.URL.Path]
		if !ok {
			fmt.Printf("[ByteMe HTTP] Route not found for: %s. Registered routes: %v\n", r.URL.Path, httpRoutes)
		}
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
			result = RunFunction(entry.closure, *vmConstantsPtr, *vmGlobalsPtr, []Object{reqMap})
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

		fmt.Printf("[ByteMe HTTP] Handler result type: %T, value: %v\n", result, result.Inspect())
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
		fmt.Printf("[ByteMe HTTP] Sending response: status=%d, contentType=%s, bodyLen=%d\n", status, contentType, len(body))
		fmt.Fprint(w, body)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("[ByteMe HTTP] Server listening on http://localhost%s\n", addr)
	err := http.ListenAndServe(addr, mux)
	fmt.Printf("[ByteMe HTTP] Server exited: %v\n", err)
	if err != nil {
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

// builtinHTTPDo performs an arbitrary HTTP request.
// httpDo(method: string, url: string, body: string, headers: map) -> map{status, body, headers}
func builtinHTTPDo(args ...Object) Object {
	if len(args) < 2 {
		return &Error{Message: "httpDo requires (method, url) arguments"}
	}
	method, ok1 := args[0].(*String)
	urlStr, ok2 := args[1].(*String)
	if !ok1 || !ok2 {
		return &Error{Message: "httpDo: method and url must be strings"}
	}

	var bodyReader io.Reader
	if len(args) >= 3 && args[2] != nil && args[2].Type() != NULL_OBJ {
		if bodyStr, ok := args[2].(*String); ok {
			bodyReader = strings.NewReader(bodyStr.Value)
		} else {
			// Fallback to inspect if not a string but passed
			bodyReader = strings.NewReader(args[2].Inspect())
		}
	}

	req, err := http.NewRequest(strings.ToUpper(method.Value), urlStr.Value, bodyReader)
	if err != nil {
		return &Error{Message: "httpDo: " + err.Error()}
	}

	// Default Content-Type for POST/PUT/PATCH if body is present
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Handle headers if provided
	if len(args) >= 4 {
		if headersMap, ok := args[3].(*Map); ok {
			for k, v := range headersMap.Pairs {
				req.Header.Set(k, v.Value.Inspect())
			}
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &Error{Message: "httpDo: " + err.Error()}
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
