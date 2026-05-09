package request

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	auth "github.com/nicklasjeppesen/going_internal/super/auth"
	. "github.com/nicklasjeppesen/going_internal/super/result"
	. "github.com/nicklasjeppesen/going_internal/super/validation"
)

type Requestbase struct {
	W http.ResponseWriter // index 0, DO NOT REORDER
	R *http.Request       // index 1, DO NOT REORDER
}

/**
 * If input is empty, it print the Request Body
 * Else loop through, and print the values.
 */
func (r *Requestbase) PrintJson(values ...any) {

	r.W.Header().Set("Content-Type", "application/json")
	for _, val := range values {
		json.NewEncoder(r.W).Encode(val)
	}
}

func (r *Requestbase) Auth() auth.Auth {

	userId := r.R.Context().Value("Auth_Id")
	if userId != nil {
		return auth.Auth{ID: userId.(string), W: r.W}
	}
	return auth.Auth{ID: "", W: r.W}
}

/**
* Validate a structs validation
* Return (bool, string)
* bool: symbolize if an error happen.
* string: error message.
 */
func (r *Requestbase) Validate(body interface{}) (bool, string) {
	return Validate(body)
}

/**
* Create a request struct, and try to parse the
* request body to the given T type.
 */
type RequestBodybase[T any] struct {
	Requestbase   // index 0, DO NOT REORDER
	Body        T // index 1, DO NOT REORDER
}

func (r *RequestBodybase[T]) GetBody() T {
	return r.Body
}

func (r *RequestBodybase[T]) Validate() *Result[T] {

	if err, errorMessage := r.validate(); err {
		return &Result[T]{Data: r.Body, Error: true, ErrorMessage: errors.New(errorMessage)}
	}

	if err := Customvalidation(r.Body); err != nil {
		return &Result[T]{Data: r.Body, Error: true, ErrorMessage: err}
	}

	return &Result[T]{Data: r.Body, Error: false, ErrorMessage: nil}
}

func (r *RequestBodybase[T]) validate() (bool, string) {
	return Validate(r.Body)
}

/**
 * If input is empty, it print the Request Body
 * Else loop through, and print the values.
 */
func (r *RequestBodybase[T]) PrintJson(values ...any) {

	r.W.Header().Set("Content-Type", "application/json")
	if len(values) == 0 {
		json.NewEncoder(r.W).Encode(r.Body)
	} else {
		for _, val := range values {
			json.NewEncoder(r.W).Encode(val)
		}
	}
}

// Cache only the reflect.Type slices, NOT reflect.Value
var typeCache sync.Map // thread safe map[string][]reflect.Type

func getParamTypes(fnValue reflect.Value) []reflect.Type {

	fnKey := fnValue.Pointer() // More stable than .String()

	var paramTypes []reflect.Type
	if cached, ok := typeCache.Load(fnKey); ok {
		paramTypes = cached.([]reflect.Type)
	} else {
		// Extract parameter types of the function, and cache them.
		fnType := fnValue.Type()
		numIn := fnType.NumIn()
		paramTypes = make([]reflect.Type, numIn)
		for i := 0; i < numIn; i++ {
			paramTypes[i] = fnType.In(i)
		}
		typeCache.Store(fnKey, paramTypes)
	}
	return paramTypes
}

/**
 *
 * Tanke: Lav en list som kan holde alle metodetyper: []interface{}
 * Dvs. lav en metode som kan håndtere structs, int, string, bool, og parse them.
 * alle metoder skal returnere val err, til err håndtering,
 * Dermed hvis det er i cache, kan man loop igennem alle parameter index 1 gang, og smide dem ind i
 * i den liste af function som passer til dem, så slipper man for foreach, case osv.
 * dvs. en dic bestående key:string = functionsnavn, value: []interface{} - liste af functioner, til at håndtere,
 * input parameter.
 * Svært, da værdier parameter kommer fra: body, param.
 * UDFØRELSE:
 * først hentes de cachede type værdier det er en liste.
 * OG SÅ SKAL DER LAVES EN SAMLET ARGLISTE I RÆKKEFØLGE: JsonData, FIBER.CTX, ...PARAMS
 * SÅ loppes der igennem alle typer, og for param, sættes en index, og en handler
 * således, at næste gang, der loppes igennem typer, så kalder paramtyper index i ARGLISTEN, får værdien og sætter ind I sin gemte handler.
 */
func CallUnknownFunc(fn interface{}, argStrings []string, w http.ResponseWriter, r *http.Request) {
	fnValue := reflect.ValueOf(fn) // get the value of the function
	//fnKey := fnValue.Pointer()     // More stable than .String()
	var paramTypes = getParamTypes(fnValue)

	in := make([]reflect.Value, len(paramTypes))
	regulator := 0

	//rResponType := reflect.TypeOf((*http.Request)(nil))

	for i, paramType := range paramTypes {

		switch {
		case paramType.Name() == "ResponseWriter": // Since pointer to an interface, We have to check by name.
			in[i] = reflect.ValueOf(w)
			regulator++
		case paramType == reflect.TypeOf((*http.Request)(nil)):
			in[i] = reflect.ValueOf(r)
			regulator++

		case paramType.Kind() == reflect.Struct || paramType.Kind() == reflect.Pointer:
			err, value := handleStructValue(w, r, paramType)
			if err {
				return
			} // return / stop  of an error occur.

			in[i] = value
			regulator++

		default:

			if len(argStrings) == 0 {
				break
			}
			argStr := argStrings[i-regulator] // what if nil?
			switch paramType.Kind() {
			case reflect.Int:
				parsed, err := strconv.Atoi(argStr)
				if err != nil {
					http.Error(w, "Invalid integer parameter", http.StatusBadRequest)
					return
				}
				in[i] = reflect.ValueOf(parsed)

			case reflect.String:
				in[i] = reflect.ValueOf(argStr)

			default:
				http.Error(w, "unsupported parameter type: "+paramType.String(), http.StatusBadRequest)
				return
			}
		}
	}

	var returnValues = fnValue.Call(in)
	handleReturnValues(returnValues, w, r)
}

func handleStructValue(w http.ResponseWriter, r *http.Request, paramType reflect.Type) (bool, reflect.Value) {

	switch {
	case strings.HasPrefix(paramType.Name(), "RequestBodybase["):
		return handleRequestBody(w, r, paramType)
	case strings.HasPrefix(paramType.Name(), "Requestbase"):
		return handleRequest(w, r, paramType)
	}
	return true, reflect.Value{}
}

func handleRequest(w http.ResponseWriter, r *http.Request, paramType reflect.Type) (bool, reflect.Value) {

	meta := reflect.New(paramType).Elem()

	// Use FieldByIndex (faster, no string lookup)
	meta.FieldByIndex([]int{0}).Set(reflect.ValueOf(w))
	meta.FieldByIndex([]int{1}).Set(reflect.ValueOf(r))
	return false, meta
}

func handleRequestBody(w http.ResponseWriter, r *http.Request, paramType reflect.Type) (bool, reflect.Value) {

	field, ok := paramType.FieldByName("Body")
	if !ok {
		panic("Struct parameter must have a 'Value' field")
	}

	strct := reflect.New(field.Type)
	m, ok := strct.Type().MethodByName("DB")
	if ok && m.IsExported() && strct.Elem().CanAddr() { // ptr. is a pointer from new method, so no need for

		methodCaller := strct.Elem().Addr().MethodByName("DB")
		if methodCaller.IsValid() && methodCaller.Type().NumIn() == 0 {
			MethodArrayResult := methodCaller.Call(nil)
			result := MethodArrayResult[0]
			strct = result
		}
	}
	err, jsonData := getRequestBody(r)
	if err != nil {
		http.Error(w, "Error reading body with error: "+err.Error(), http.StatusInternalServerError)
		return true, reflect.Value{}
	}

	// STEP 1: Check Keys:
	var keyMap map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &keyMap); err != nil {
		http.Error(w, "Invalid Json format", http.StatusBadRequest)

		return true, reflect.Value{}
	}
	innerType := field.Type
	for i := 0; i < innerType.NumField(); i++ {
		tag := innerType.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if _, ok := keyMap[tag]; !ok {
			http.Error(w, "Missing key: "+tag, http.StatusBadRequest)
			return true, reflect.Value{}
		}
	}

	// STEP 1.2: Unmarshall - add request data to struct:
	if err := json.Unmarshal([]byte(jsonData), strct.Interface()); err != nil {
		http.Error(w, "Invalid integer parameter", http.StatusBadRequest)
		return true, reflect.Value{}
	}

	// Step 2: Build The requestBody struct
	requestBody := reflect.New(paramType).Elem()

	// Use FieldByIndex (faster, no string lookup)
	requestBody.FieldByIndex([]int{0}).Set(reflect.ValueOf(Requestbase{w, r}))
	requestBody.FieldByIndex([]int{1}).Set(strct.Elem()) // Know Value T index is 2
	return false, requestBody
}

func getRequestBody(r *http.Request) (error, []byte) {

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return err, nil
	}
	defer r.Body.Close() // Always close the body

	return nil, bodyBytes
}

/*
*
* Assume the return type is always a type of func(http.ResponseWriter, *http.Request)
 */
func handleReturnValues(returnvalues []reflect.Value, w http.ResponseWriter, r *http.Request) {

	// Checking if return type is a correct reponse type.
	if len(returnvalues) >= 1 {
		if fn, ok := returnvalues[0].Interface().(func(http.ResponseWriter, *http.Request)); ok {
			fn(w, r)
		}
	}
}
