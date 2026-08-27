package request

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// Cache only the reflect.Type slices, NOT reflect.Value
//var typeCache sync.Map // thread safe map[string][]reflect.Type

/*
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
}*/

type ArgHandler func(w http.ResponseWriter, r *http.Request, argStrings []string, argIndex *int) (reflect.Value, error)

type structMeta struct {
	bodyFieldType reflect.Type
	bodyIsPointer bool
	requiredTags  []string // pre-parsed json tags til validering
}

var planCache sync.Map       // map[uintptr][]ArgHandler
var structMetaCache sync.Map // map[reflect.Type]*structMeta

func getCallPlan(fnValue reflect.Value) []ArgHandler {
	fnKey := fnValue.Pointer()

	if cached, ok := planCache.Load(fnKey); ok {
		return cached.([]ArgHandler)
	}

	fnType := fnValue.Type()
	plan := make([]ArgHandler, 0, fnType.NumIn())

	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		plan = append(plan, buildHandler(paramType))
	}

	planCache.Store(fnKey, plan)
	return plan
}

func buildHandler(paramType reflect.Type) ArgHandler {
	switch {
	case paramType.Name() == "ResponseWriter":
		return func(w http.ResponseWriter, r *http.Request, _ []string, _ *int) (reflect.Value, error) {
			return reflect.ValueOf(w), nil
		}

	case paramType == reflect.TypeOf((*http.Request)(nil)):
		return func(_ http.ResponseWriter, r *http.Request, _ []string, _ *int) (reflect.Value, error) {
			return reflect.ValueOf(r), nil
		}

	case strings.HasPrefix(paramType.Name(), "RequestBodybase["):
		meta := getStructMeta(paramType) // cachet metadata
		return func(w http.ResponseWriter, r *http.Request, _ []string, _ *int) (reflect.Value, error) {
			return handleRequestBodyWithMeta(w, r, paramType, meta)
		}

	case strings.HasPrefix(paramType.Name(), "Requestbase"):
		return func(w http.ResponseWriter, r *http.Request, _ []string, _ *int) (reflect.Value, error) {
			_, v := handleRequest(w, r, paramType)
			return v, nil
		}

	case paramType.Kind() == reflect.Int:
		return func(_ http.ResponseWriter, _ *http.Request, argStrings []string, argIndex *int) (reflect.Value, error) {
			if *argIndex >= len(argStrings) {
				return reflect.Value{}, fmt.Errorf("missing parameter")
			}
			v, err := strconv.Atoi(argStrings[*argIndex])
			*argIndex++
			if err != nil {
				return reflect.Value{}, fmt.Errorf("invalid integer parameter")
			}
			return reflect.ValueOf(v), nil
		}

	case paramType.Kind() == reflect.String:
		return func(_ http.ResponseWriter, _ *http.Request, argStrings []string, argIndex *int) (reflect.Value, error) {
			if *argIndex >= len(argStrings) {
				return reflect.Value{}, fmt.Errorf("missing parameter")
			}
			v := argStrings[*argIndex]
			*argIndex++
			return reflect.ValueOf(v), nil
		}
	}

	// fallback for evt. andre structs
	return func(w http.ResponseWriter, r *http.Request, _ []string, _ *int) (reflect.Value, error) {
		_, v := handleStructValue(w, r, paramType)
		return v, nil
	}
}

func handleRequestBodyWithMeta(w http.ResponseWriter, r *http.Request, requestBodyStruct reflect.Type, meta *structMeta) (reflect.Value, error) {

	// Create a new instance of the ORM struct (this part CANNOT be cached, see caveat)
	elemType := meta.bodyFieldType
	if meta.bodyIsPointer {
		elemType = elemType.Elem()
	}
	ormStruct := reflect.New(elemType)

	// Parse body ind i den nye struct
	if err := parseDataToOrm(r, ormStruct.Interface()); err != nil {
		return reflect.Value{}, err
	}

	// Build the requestBody Wrapper
	requestBody := reflect.New(requestBodyStruct).Elem()
	requestBody.Field(0).Set(reflect.ValueOf(Requestbase{w, r}))

	if meta.bodyIsPointer {
		requestBody.Field(1).Set(ormStruct)
	} else {
		requestBody.Field(1).Set(ormStruct.Elem())
	}

	return requestBody, nil
}

func CallUnknownFunc(fn interface{}, argStrings []string, w http.ResponseWriter, r *http.Request) {
	fnValue := reflect.ValueOf(fn)
	plan := getCallPlan(fnValue)

	args := make([]reflect.Value, 0, len(plan))
	argIndex := 0

	for _, handler := range plan {
		v, err := handler(w, r, argStrings, &argIndex)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		args = append(args, v)
	}

	returnValues := fnValue.Call(args)
	handleReturnValues(returnValues, w, r)
}

func getStructMeta(t reflect.Type) *structMeta {
	if cached, ok := structMetaCache.Load(t); ok {
		return cached.(*structMeta)
	}

	field, _ := t.FieldByName("Body")
	bodyType := field.Type
	isPtr := bodyType.Kind() == reflect.Ptr

	elemType := bodyType
	if isPtr {
		elemType = bodyType.Elem()
	}

	tags := make([]string, 0, elemType.NumField())
	for i := 0; i < elemType.NumField(); i++ {
		tag := strings.Split(elemType.Field(i).Tag.Get("json"), ",")[0]
		if tag != "" && tag != "-" {
			tags = append(tags, tag)
		}
	}

	meta := &structMeta{bodyFieldType: bodyType, bodyIsPointer: isPtr, requiredTags: tags}
	structMetaCache.Store(t, meta)
	return meta
}

/*
func CallUnknownFunc(fn interface{}, argStrings []string, w http.ResponseWriter, r *http.Request) {
	fnValue := reflect.ValueOf(fn)
	paramTypes := getParamTypes(fnValue)

	args := make([]reflect.Value, 0, len(paramTypes))
	argIndex := 0

	for _, paramType := range paramTypes {

		// Inject ResponseWriter
		if paramType.Name() == "ResponseWriter" {
			args = append(args, reflect.ValueOf(w))
			continue
		}

		// Inject *http.Request
		if paramType == reflect.TypeOf((*http.Request)(nil)) {
			args = append(args, reflect.ValueOf(r))
			continue
		}

		// Handle structs/pointers
		if paramType.Kind() == reflect.Struct ||
			paramType.Kind() == reflect.Pointer {

			err, value := handleStructValue(w, r, paramType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			args = append(args, value)
			continue
		}

		// Normal URL/string arguments
		if argIndex >= len(argStrings) {
			http.Error(w, "missing parameter", http.StatusBadRequest)
			return
		}

		value, err := parseArgument(argStrings[argIndex], paramType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		args = append(args, value)
		argIndex++
	}

	returnValues := fnValue.Call(args)
	handleReturnValues(returnValues, w, r)
}
*/

/*
func parseArgument(arg string, t reflect.Type) (reflect.Value, error) {

	switch t.Kind() {

	case reflect.Int:
		v, err := strconv.Atoi(arg)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid integer parameter")
		}
		return reflect.ValueOf(v), nil

	case reflect.String:
		return reflect.ValueOf(arg), nil

	default:
		return reflect.Value{}, fmt.Errorf(
			"unsupported parameter type: %s",
			t.String(),
		)
	}
}
*/

func handleStructValue(w http.ResponseWriter, r *http.Request, paramType reflect.Type) (error, reflect.Value) {
	switch {
	case strings.HasPrefix(paramType.Name(), "RequestBodybase["):
		return handleRequestBody(w, r, paramType)
	case strings.HasPrefix(paramType.Name(), "Requestbase"):
		return handleRequest(w, r, paramType)
	}
	return errors.New("wrong input type"), reflect.Value{}
}

func handleRequest(w http.ResponseWriter, r *http.Request, paramType reflect.Type) (error, reflect.Value) {
	meta := reflect.New(paramType).Elem()
	meta.FieldByIndex([]int{0}).Set(reflect.ValueOf(w))
	meta.FieldByIndex([]int{1}).Set(reflect.ValueOf(r))
	return nil, meta
}

func handleRequestBody(w http.ResponseWriter, r *http.Request, requestBodyStruct reflect.Type) (error, reflect.Value) {
	if err, requestBodyField := getRequestBodyFieldBody(w, r, requestBodyStruct); err != nil {
		fmt.Println("Error in hanndleRequestBody")
		return err, reflect.Value{}
	} else {
		return nil, buildRequestBody(requestBodyStruct, w, r, requestBodyField)
	}
}

func getRequestBodyFieldBody(w http.ResponseWriter, r *http.Request, requestBodyStruct reflect.Type) (error, reflect.Value) {
	if err, requestBodyField := getRequestBodyField(requestBodyStruct, w); err != nil {
		return err, reflect.Value{}
	} else {
		return requestBodyFieldWithData(requestBodyField, r)
	}
}

func getRequestBodyField(requestBodyStruct reflect.Type, w http.ResponseWriter) (error, reflect.StructField) {
	field, ok := requestBodyStruct.FieldByName("Body")
	if !ok {
		return errors.New("Struct must contain Body field"), reflect.StructField{}
	}
	return nil, field
}

func requestBodyFieldWithData(requestBodyField reflect.StructField, r *http.Request) (error, reflect.Value) {
	if err, ormStruct := createOrmStruct(requestBodyField.Type); err != nil {
		return err, reflect.Value{}
	} else {
		err = parseDataToOrm(r, ormStruct.Interface())

		return err, ormStruct
	}
}

func createOrmStruct(ormStructReflect reflect.Type) (error, reflect.Value) {

	if ormStructReflect.Kind() == reflect.Ptr {
		ormStructReflect = ormStructReflect.Elem()
	}
	ormStruct := reflect.New(ormStructReflect)
	return nil, ormStruct
}

/*
func validateRequiredJSONFields(body []byte, t reflect.Type) error {
	var keyMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &keyMap); err != nil {
		return fmt.Errorf("invalid JSON format")
	}

	for i := 0; i < t.NumField(); i++ {

		tag := t.Field(i).Tag.Get("json")
		tag = strings.Split(tag, ",")[0] // Foreslået af AI, skal lige tjekkes

		if tag == "" || tag == "-" {
			continue
		}

		if _, ok := keyMap[tag]; !ok {
			return fmt.Errorf("missing key: %s", tag)
		}
	}

	return nil
}*/

func buildRequestBody(paramType reflect.Type, w http.ResponseWriter, r *http.Request, body reflect.Value) reflect.Value {
	requestBody := reflect.New(paramType).Elem()
	requestBody.Field(0).Set(reflect.ValueOf(Requestbase{w, r}))

	if requestBody.Field(1).Type().Kind() == reflect.Ptr {
		requestBody.Field(1).Set(body)
	} else {
		requestBody.Field(1).Set(body.Elem())
	}
	return requestBody
}

// Assume the return type is always a type of func(http.ResponseWriter, *http.Request)
func handleReturnValues(returnvalues []reflect.Value, w http.ResponseWriter, r *http.Request) {

	// Checking if return type is a correct reponse type.
	if len(returnvalues) >= 1 {
		if fn, ok := returnvalues[0].Interface().(func(http.ResponseWriter, *http.Request)); ok {
			fn(w, r)
		} // others?
	}
}
