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
package main

// EmptyString
var EmptyString = newString("")

func newString(s string) *LOB {
	str := newLOB(typeString)
	str.text = s
	return str
}

func asString(obj *LOB) (string, error) {
	if !isString(obj) {
		return "", TypeError(typeString, obj)
	}
	return obj.text, nil
}

func toString(a *LOB) (*LOB, error) {
	switch a.variant {
	case typeCharacter:
		return newString(string([]rune{rune(a.ival)})), nil
	case typeString:
		return a, nil
	case typeSymbol, typeKeyword, typeType:
		return newString(a.text), nil
	case typeNumber, typeBoolean:
		return newString(a.String()), nil
	case typeVector:
		var chars []rune
		for _, c := range a.elements {
			if !isCharacter(c) {
				return nil, Error("to-string: vector element is not a <character>: ", c)
			}
			chars = append(chars, rune(c.ival))
		}
		return newString(string(chars)), nil
	case typeList:
		var chars []rune
		for a != EmptyList {
			c := car(a)
			if !isCharacter(c) {
				return nil, Error("to-string: list element is not a <character>: ", c)
			}
			chars = append(chars, rune(c.ival))
			a = a.cdr
		}
		return newString(string(chars)), nil
	default:
		return nil, Error("to-string: cannot convert argument to <string>: ", a)
	}
}

func stringLength(s string) int {
	count := 0
	for range s {
		count++
	}
	return count
}

func encodeString(s string) string {
	buf := []rune{}
	buf = append(buf, '"')
	for _, c := range s {
		switch c {
		case '"':
			buf = append(buf, '\\')
			buf = append(buf, '"')
		case '\\':
			buf = append(buf, '\\')
			buf = append(buf, '\\')
		case '\n':
			buf = append(buf, '\\')
			buf = append(buf, 'n')
		case '\t':
			buf = append(buf, '\\')
			buf = append(buf, 't')
		case '\f':
			buf = append(buf, '\\')
			buf = append(buf, 'f')
		case '\b':
			buf = append(buf, '\\')
			buf = append(buf, 'b')
		case '\r':
			buf = append(buf, '\\')
			buf = append(buf, 'r')
		default:
			buf = append(buf, c)
		}
	}
	buf = append(buf, '"')
	return string(buf)
}

func newCharacter(c rune) *LOB {
	char := newLOB(typeCharacter)
	char.ival = int64(c)
	return char
}

func asCharacter(c *LOB) (rune, error) {
	if !isCharacter(c) {
		return 0, TypeError(typeCharacter, c)
	}
	return rune(c.ival), nil
}

func stringCharacters(s *LOB) []*LOB {
	var chars []*LOB
	for _, c := range s.text {
		chars = append(chars, newCharacter(c))
	}
	return chars
}

func stringToVector(s *LOB) *LOB {
	return vector(stringCharacters(s)...)
}

func stringToList(s *LOB) *LOB {
	return list(stringCharacters(s)...)
}
