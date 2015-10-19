/*
Copyright 2014 Lee Boynton

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ell

import (
	"bytes"
	"fmt"
	"strconv"
)

// LOB type is the Ell object: a union of all possible primitive types. Which fields are used depends on the variant
// the variant is a type object i.e. intern("<string>")
type LOB struct {
	Type         *LOB          // i.e. <string>
	code         *Code         // non-nil for closure, code
	frame        *Frame        // non-nil for closure, continuation
	primitive    *Primitive    // non-nil for primitives
	continuation *Continuation // non-nil for continuation
	car          *LOB          // non-nil for instances and lists
	cdr          *LOB          // non-nil for slists, nil for everything else
	bindings     map[Key]*LOB  // non-nil for struct
	elements     []*LOB        // non-nil for vector
	fval         float64       // number
	text         string        // string, symbol, keyword, type, blob, channel
	Value        interface{}   // the rest of the data for more complex things
}

func BoolValue(obj *LOB) bool {
	if obj == True {
		return true
	}
	return false
}

func RuneValue(obj *LOB) rune {
	return rune(obj.fval)
}

func IntValue(obj *LOB) int {
	return int(obj.fval)
}

func Int64Value(obj *LOB) int64 {
	return int64(obj.fval)
}

func Float64Value(obj *LOB) float64 {
	return obj.fval
}

func StringValue(obj *LOB) string {
	return obj.text
}

func BlobValue(obj *LOB) []byte {
	return []byte(obj.text)
}

func NewObject(variant *LOB, value interface{}) *LOB {
	lob := newLOB(variant)
	lob.Value = value
	return lob
}

func newLOB(variant *LOB) *LOB {
	lob := new(LOB)
	lob.Type = variant
	return lob
}

func Identical(o1 *LOB, o2 *LOB) bool {
	return o1 == o2
}

type stringable interface {
	String() string
}

func (lob *LOB) String() string {
	switch lob.Type {
	case NullType:
		return "null"
	case BooleanType:
		if lob == True {
			return "true"
		}
		return "false"
	case CharacterType:
		return string([]rune{rune(lob.fval)})
	case NumberType:
		return strconv.FormatFloat(lob.fval, 'f', -1, 64)
	case BlobType:
		return fmt.Sprintf("#[blob %d bytes]", len(lob.text))
	case StringType, SymbolType, KeywordType, TypeType:
		return lob.text
	case ListType:
		return listToString(lob)
	case VectorType:
		return vectorToString(lob)
	case StructType:
		return structToString(lob)
	case FunctionType:
		return functionToString(lob)
	case CodeType:
		return lob.code.String()
	case ErrorType:
		return "#<error>" + write(lob.car)
		//	case ChannelType:
		//		return ChannelValue(lob).String()
	default:
		if lob.Value != nil {
			if s, ok := lob.Value.(stringable); ok {
				return s.String()
			}
			return "#[" + typeNameString(lob.Type.text) + "]"
		}
		return "#" + lob.Type.text + write(lob.car)
	}
}

// TypeType is the metatype, the type of all types
var TypeType *LOB // bootstrapped in initSymbolTable => intern("<type>")

// KeywordType is the type of all keywords
var KeywordType *LOB // bootstrapped in initSymbolTable => intern("<keyword>")

// SymbolType is the type of all symbols
var SymbolType *LOB // bootstrapped in initSymbolTable = intern("<symbol>")

// NullType the type of the null object
var NullType = intern("<null>")

// BooleanType is the type of true and false
var BooleanType = intern("<boolean>")

// CharacterType is the type of all characters
var CharacterType = intern("<character>")

// NumberType is the type of all numbers
var NumberType = intern("<number>")

// StringType is the type of all strings
var StringType = intern("<string>")

// BlobType is the type of all bytearrays
var BlobType = intern("<blob>")

// ListType is the type of all lists
var ListType = intern("<list>")

// VectorType is the type of all vectors
var VectorType = intern("<vector>")

// VectorType is the type of all structs
var StructType = intern("<struct>")

// FunctionType is the type of all functions
var FunctionType = intern("<function>")

// CodeType is the type of compiled code
var CodeType = intern("<code>")

// ErrorType is the type of all errors
var ErrorType = intern("<error>")

// AnyType is a pseudo type specifier indicating any type
var AnyType = intern("<any>")

// Null is Ell's version of nil. It means "nothing" and is not the same as EmptyList. It is a singleton.
var Null = &LOB{Type: NullType}

func IsNull(obj *LOB) bool {
	return obj == Null
}

// True is the singleton boolean true value
var True = &LOB{Type: BooleanType, fval: 1}

// False is the singleton boolean false value
var False = &LOB{Type: BooleanType, fval: 0}

func IsBoolean(obj *LOB) bool {
	return obj.Type == BooleanType
}

func IsCharacter(obj *LOB) bool {
	return obj.Type == CharacterType
}
func IsNumber(obj *LOB) bool {
	return obj.Type == NumberType
}
func IsString(obj *LOB) bool {
	return obj.Type == StringType
}
func IsList(obj *LOB) bool {
	return obj.Type == ListType
}
func IsVector(obj *LOB) bool {
	return obj.Type == VectorType
}
func IsStruct(obj *LOB) bool {
	return obj.Type == StructType
}
func IsFunction(obj *LOB) bool {
	return obj.Type == FunctionType
}
func IsCode(obj *LOB) bool {
	return obj.Type == CodeType
}
func IsSymbol(obj *LOB) bool {
	return obj.Type == SymbolType
}
func IsKeyword(obj *LOB) bool {
	return obj.Type == KeywordType
}
func IsType(obj *LOB) bool {
	return obj.Type == TypeType
}

//instances have arbitrary Type symbols, all we can check is that the instanceValue is set
func IsInstance(obj *LOB) bool {
	return obj.car != nil && obj.cdr == nil
}

func Equal(o1 *LOB, o2 *LOB) bool {
	if o1 == o2 {
		return true
	}
	if o1.Type != o2.Type {
		return false
	}
	switch o1.Type {
	case BooleanType, CharacterType:
		return int(o1.fval) == int(o2.fval)
	case NumberType:
		return numberEqual(o1.fval, o2.fval)
	case StringType:
		return o1.text == o2.text
	case ListType:
		return listEqual(o1, o2)
	case VectorType:
		return vectorEqual(o1, o2)
	case StructType:
		return structEqual(o1, o2)
	case SymbolType, KeywordType, TypeType:
		return o1 == o2
	case NullType:
		return true // singleton
	default:
		o1a := Value(o1)
		if o1a != o1 {
			o2a := Value(o2)
			return Equal(o1a, o2a)
		}
		return false
	}
}

func IsPrimitiveType(tag *LOB) bool {
	switch tag {
	case NullType, BooleanType, CharacterType, NumberType, StringType, ListType, VectorType, StructType:
		return true
	case SymbolType, KeywordType, TypeType, FunctionType:
		return true
	default:
		return false
	}
}

func Instance(tag *LOB, val *LOB) (*LOB, error) {
	if !IsType(tag) {
		return nil, Error(ArgumentErrorKey, TypeType.text, tag)
	}
	if IsPrimitiveType(tag) {
		return val, nil
	}
	result := newLOB(tag)
	result.car = val
	return result, nil
}

func Value(obj *LOB) *LOB {
	if obj.cdr == nil && obj.car != nil {
		return obj.car
	}
	return obj
}

//
// Error - creates a new Error from the arguments. The first is an actual Ell object, the rest are interpreted as/converted to strings
//
func Error(errkey *LOB, args ...interface{}) error {
	var buf bytes.Buffer
	for _, o := range args {
		if l, ok := o.(*LOB); ok {
			buf.WriteString(fmt.Sprintf("%v", write(l)))
		} else {
			buf.WriteString(fmt.Sprintf("%v", o))
		}
	}
	if errkey.Type != KeywordType {
		errkey = ErrorKey
	}
	return newError(errkey, newString(buf.String()))
}

func newError(elements ...*LOB) *LOB {
	data := vector(elements...)
	return &LOB{Type: ErrorType, car: data}
}

func theError(o interface{}) (*LOB, bool) {
	if o == nil {
		return nil, false
	}
	if err, ok := o.(*LOB); ok {
		if err.Type == ErrorType {
			return err, true
		}
	}
	return nil, false

}

func isError(o interface{}) bool {
	_, ok := theError(o)
	return ok
}

func errorData(err *LOB) *LOB {
	return err.car
}

// Error
func (lob *LOB) Error() string {
	if lob.Type == ErrorType {
		s := lob.car.String()
		if lob.text != "" {
			s += " [in " + lob.text + "]"
		}
		return s
	}
	return lob.String()
}

// ErrorKey - used to generic errors
var ErrorKey = intern("error:")

// ArgumentErrorKey
var ArgumentErrorKey = intern("argument-error:")

// SyntaxErrorKey
var SyntaxErrorKey = intern("syntax-error:")

// MacroErrorKey
var MacroErrorKey = intern("macro-error:")

// IOErrorKey
var IOErrorKey = intern("io-error:")

// HttpErrorKey
var HTTPErrorKey = intern("http-error:")

// InterruptKey
var InterruptKey = intern("interrupt:")
