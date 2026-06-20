package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

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
}

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
