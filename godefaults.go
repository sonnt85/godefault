package godefault

import (
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sonnt85/gogmap"
)

// Applies the default values to the struct object, the struct type must have
// the StructTag with name "default" and the directed value.
//
// Usage
//
//	type ExampleBasic struct {
//	    Foo bool   `default:"true"`
//	    Bar string `default:"33"`
//	    Qux int8
//	    Dur time.Duration `default:"2m3s"`
//	}
//
//	 foo := &ExampleBasic{}
//	 SetDefaults(foo)
func SetDefaults(variable interface{}, tagNames ...string) {
	getDefaultFiller(tagNames...).Fill(variable)
}

var defaultFiller *Filler = nil

func getDefaultFiller(tagNames ...string) *Filler {
	if defaultFiller == nil {
		defaultFiller = newDefaultFiller(tagNames...)
	}

	return defaultFiller
}

// parseEnvString performs parsing of an input string based on a specific format
// and returns the corresponding value based on the following rules:
//
// The input string is expected to be in the following format:
//
//			envs|[envkey|]env1,env1_value_[base64]|env2,,env2_value_[base64]|...
//
//		  - The input string begins with the "envs|" prefix, which indicates that it contains
//		    a list of environment variable definitions.
//	  	  - If it is Envs then chain |, encrypt, and || encryption for |
//		  - Each environment variable definition is separated by the "|" character.
//		  - Each definition consists of two or three parts, separated by commas:
//		  - The first part is the environment variable key (envkey).
//		  - The second part is the environment variable value (env1_value).
//		  - Optionally, the third part is a base64-encoded value enclosed in square brackets,
//		    e.g., "[base64]", which indicates that the value needs to be base64 decoded.
//
// The function will return the value of the specified environment variable, and if it's
// base64 encoded, it will decode the value before returning it.
//
// Input parameters:
// - envStr: The environment variable string to parse.
//
// Return value:
// - retstr: The value of the parsed environment variable, or a default value if not found.
func parseEnvString(envStr string) (retstr string) {
	// envs|[envkey|]env1,env1_value_[base64]|env2,,env2_value_[base64]|...
	retstr = envStr
	prefix := "envs|"
	if strings.HasPrefix(envStr, prefix) {
		envStr = envStr[len(prefix):]
	} else {
		return
	}
	envStrTmp := strings.ReplaceAll(envStr, "|,", "__orcomma__")
	envStrTmp = strings.ReplaceAll(envStrTmp, "||", "__oror__")
	parts := strings.Split(envStrTmp, "|")
	if envStrTmp != envStr {
		defer func() {
			retstr = strings.ReplaceAll(retstr, "__oror__", "|")
			retstr = strings.ReplaceAll(retstr, "__orcomma__", ",")
		}()
	}
	// parts := strings.Split(envStr, "|")
	if len(parts) < 2 {
		return
	}

	// Extract the environment key from the first part (before the colon)
	separatorChar := ","

	key := parts[0]
	if strings.Contains(key, separatorChar) {
		key = "EnvType"
	} else {
		parts = parts[1:]
	}
	// value := os.Getenv(key)
	value := gogmap.Get(key)
	if value == "" {
		value = os.Getenv(key)
	}
	defaultValue := ""
	// Loop through the remaining parts to find the matching environment variable
	for i, part := range parts {
		envValues := strings.Split(part, separatorChar)

		if len(envValues) != 2 && len(envValues) != 3 {
			return
		}
		if i == 0 {
			defaultValue = envValues[1]
			if value == "" {
				value = envValues[0]
			}
		}
		// Check if the current environment key matches the specified key
		if envValues[0] == value {
			if len(envValues) == 3 {
				if envValues[1] == "" {
					if decodedBytes, err := base64.StdEncoding.DecodeString(envValues[2]); err == nil {
						return string(decodedBytes)
					}
				}
				return
			}
			retstr = envValues[1]
			return

		}
	}
	retstr = defaultValue
	return
}

// parseDateTimeString parses a string consisting of two parts: a layout and a time value.
// If a layout is provided, it uses that layout to parse the time value. If no layout is
// provided, it uses the default layout "2006-01-02 15:04:05".
func parseDateTime(dateTimeString string) (time.Time, error) {
	// Split the string into layout and value using a space as the separator
	// 	parts := strings.Split(dateTimeString, " ")
	parts := strings.Fields(dateTimeString)

	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid string: %s", dateTimeString)
	}

	layout := "2006-01-02 15:04:05"
	value := dateTimeString
	if len(parts) > 2 {
		layout = strings.Join(parts[len(parts)/2:], " ")
		value = strings.Join(parts[:len(parts)/2], " ")
	}

	parsedTime, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, err
	}

	return parsedTime, nil
}

func newDefaultFiller(tagNames ...string) *Filler {
	funcs := make(map[reflect.Kind]FillerFunc, 0)
	funcs[reflect.Bool] = func(field *FieldData) {
		value, _ := strconv.ParseBool(field.TagValue)
		field.Value.SetBool(value)
	}

	funcs[reflect.Int] = func(field *FieldData) {
		value, _ := strconv.ParseInt(field.TagValue, 10, 64)
		field.Value.SetInt(value)
	}

	funcs[reflect.Int8] = funcs[reflect.Int]
	funcs[reflect.Int16] = funcs[reflect.Int]
	funcs[reflect.Int32] = funcs[reflect.Int]
	funcs[reflect.Int64] = func(field *FieldData) {
		if field.Field.Type == reflect.TypeOf(time.Second) {
			value, _ := time.ParseDuration(field.TagValue)
			field.Value.Set(reflect.ValueOf(value))
		} else {
			value, _ := strconv.ParseInt(field.TagValue, 10, 64)
			field.Value.SetInt(value)
		}
	}

	funcs[reflect.Float32] = func(field *FieldData) {
		value, _ := strconv.ParseFloat(field.TagValue, 64)
		field.Value.SetFloat(value)
	}

	funcs[reflect.Float64] = funcs[reflect.Float32]

	funcs[reflect.Uint] = func(field *FieldData) {
		value, _ := strconv.ParseUint(field.TagValue, 10, 64)
		field.Value.SetUint(value)
	}

	funcs[reflect.Uint8] = funcs[reflect.Uint]
	funcs[reflect.Uint16] = funcs[reflect.Uint]
	funcs[reflect.Uint32] = funcs[reflect.Uint]
	funcs[reflect.Uint64] = funcs[reflect.Uint]

	funcs[reflect.String] = func(field *FieldData) {
		if field.TagValue == "-," {
			field.TagValue = "-"
		}
		tagValue := parseEnvString(field.TagValue)
		if tagValue == field.TagValue {
			tagValue = parseDateTimeString(field.TagValue)
		}
		field.Value.SetString(tagValue)
	}

	funcs[reflect.Struct] = func(field *FieldData) {
		fields := getDefaultFiller().GetFieldsFromValue(field.Value, nil)
		getDefaultFiller().SetDefaultValues(fields)
	}

	types := make(map[TypeHash]FillerFunc, 1)
	types["time.Duration"] = func(field *FieldData) {
		d, _ := time.ParseDuration(field.TagValue)
		field.Value.Set(reflect.ValueOf(d))
	}
	types["time.Time"] = func(field *FieldData) {
		d, _ := parseDateTime(field.TagValue)
		field.Value.Set(reflect.ValueOf(d))
	}
	funcs[reflect.Slice] = func(field *FieldData) {
		k := field.Value.Type().Elem().Kind()
		switch k {
		case reflect.Uint8:
			if field.Value.Bytes() != nil {
				return
			}
			field.Value.SetBytes([]byte(field.TagValue))
		case reflect.Struct:
			count := field.Value.Len()
			for i := 0; i < count; i++ {
				fields := getDefaultFiller().GetFieldsFromValue(field.Value.Index(i), nil)
				getDefaultFiller().SetDefaultValues(fields)
			}
		default:
			//处理形如 [1,2,3,4]
			reg := regexp.MustCompile(`^\[(.*)\]$`)
			matchs := reg.FindStringSubmatch(field.TagValue)
			if len(matchs) != 2 {
				return
			}
			if matchs[1] == "" {
				field.Value.Set(reflect.MakeSlice(field.Value.Type(), 0, 0))
			} else {
				match1 := strings.ReplaceAll(matchs[1], "|,", "__orcomma__") //
				defaultValue := strings.Split(match1, ",")
				result := reflect.MakeSlice(field.Value.Type(), len(defaultValue), len(defaultValue))
				for i := 0; i < len(defaultValue); i++ {
					itemValue := result.Index(i)
					defaultValue[i] = strings.ReplaceAll(defaultValue[i], "__orcomma__", ",")
					item := &FieldData{
						Value:    itemValue,
						Field:    reflect.StructField{},
						TagValue: defaultValue[i],
						Parent:   nil,
					}
					funcs[k](item)
				}
				field.Value.Set(result)
			}
		}
	}
	tagname := "default"
	if len(tagNames) != 0 {
		tagname = tagNames[0]
	}
	return &Filler{FuncByKind: funcs, FuncByType: types, Tag: tagname}
}

func parseDateTimeString(data string) string {

	pattern := regexp.MustCompile(`\{\{(\w+\:(?:-|)\d*,(?:-|)\d*,(?:-|)\d*)\}\}`)
	matches := pattern.FindAllStringSubmatch(data, -1) // matches is [][]string
	for _, match := range matches {

		tags := strings.Split(match[1], ":")
		if len(tags) == 2 {

			valueStrings := strings.Split(tags[1], ",")
			if len(valueStrings) == 3 {
				var values [3]int
				for key, valueString := range valueStrings {
					num, _ := strconv.ParseInt(valueString, 10, 64)
					values[key] = int(num)
				}

				switch tags[0] {

				case "date":
					str := time.Now().AddDate(values[0], values[1], values[2]).Format("2006-01-02")
					data = strings.Replace(data, match[0], str, -1)
					break
				case "time":
					str := time.Now().Add((time.Duration(values[0]) * time.Hour) +
						(time.Duration(values[1]) * time.Minute) +
						(time.Duration(values[2]) * time.Second)).Format("15:04:05")
					data = strings.Replace(data, match[0], str, -1)
					break
				}
			}
		}

	}
	return data
}
