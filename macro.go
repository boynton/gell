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

import (
	"fmt"
)

type macro struct {
	name     LAny
	expander LAny //a function of one argument
}

func newMacro(name LAny, expander LAny) *macro {
	return &macro{name, expander}
}

func (mac *macro) String() string {
	return fmt.Sprintf("(macro %v %v)", mac.name, mac.expander)
}

func macroexpand(expr LAny) (LAny, error) {
	return macroexpandObject(expr)
}

func macroexpandObject(expr LAny) (LAny, error) {
	if lst, ok := expr.(*LList); ok {
		if lst != EmptyList {
			return macroexpandList(lst)
		}
	}
	return expr, nil
}

func macroexpandList(expr *LList) (LAny, error) {
	if expr == EmptyList {
		return expr, nil
	}
	lst := expr
	fn := car(lst)
	head := LAny(fn)
	if isSymbol(fn) {
		result, err := expandPrimitive(fn, lst)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
		head = fn
	} else if isList(fn) {
		expanded, err := macroexpandList(fn.(*LList))
		if err != nil {
			return nil, err
		}
		head = expanded
	}
	tail, err := expandSequence(cdr(expr))
	if err != nil {
		return nil, err
	}
	return cons(head, tail), nil
}

func (mac *macro) expand(expr *LList) (LAny, error) {
	expander := mac.expander
	switch fun := expander.(type) {
	case *LClosure:
		if fun.code.argc == 1 {
			expanded, err := exec(fun.code, expr)
			if err == nil {
				if result, ok := expanded.(*LList); ok {
					return macroexpandObject(result)
				}
				return expanded, err
			}
			return nil, Error("macro error in '", mac.name, "': ", err)
		}
	case *LPrimitive:
		args := []LAny{expr}
		expanded, err := fun.fun(args, 1)
		if err == nil {
			return macroexpandObject(expanded)
		}
		return nil, err
	}
	return nil, Error("Bad macro expander function")
}

func expandSequence(seq *LList) (*LList, error) {
	var result []LAny
	if seq == nil {
		panic("Whoops: should be (), not nil!")
	}
	for seq != EmptyList {
		switch item := car(seq).(type) {
		case *LList:
			expanded, err := macroexpandList(item)
			if err != nil {
				return nil, err
			}
			result = append(result, expanded)
		default:
			result = append(result, item)
		}
		seq = cdr(seq)
	}
	lst := toList(result)
	if seq != EmptyList {
		tmp := cons(seq, EmptyList)
		return concat(lst, tmp)
	}
	return lst, nil
}

func expandIf(expr LAny) (*LList, error) {
	i := length(expr)
	if i == 4 {
		tmp, err := expandSequence(cdr(expr))
		if err != nil {
			return nil, err
		}
		return cons(car(expr), tmp), nil
	} else if i == 3 {
		tmp := list(cadr(expr), caddr(expr), Null)
		tmp, err := expandSequence(tmp)
		if err != nil {
			return nil, err
		}
		return cons(car(expr), tmp), nil
	} else {
		return nil, SyntaxError(expr)
	}
}

func expandUndefine(expr *LList) (*LList, error) {
	if length(expr) != 2 || !isSymbol(cadr(expr)) {
		return nil, SyntaxError(expr)
	}
	return expr, nil
}

func expandDefine(expr *LList) (LAny, error) {
	exprLen := length(expr)
	if exprLen < 3 {
		return nil, SyntaxError(expr)
	}
	name := cadr(expr)
	if isSymbol(name) {
		if exprLen > 3 {
			return nil, SyntaxError(expr)
		}
		body, ok := caddr(expr).(*LList)
		if !ok {
			return expr, nil
		}
		val, err := macroexpandList(body)
		if err != nil {
			return nil, err
		}
		return list(car(expr), name, val), nil
	} else if isList(name) {
		args := cdr(name)
		name = car(name)
		body, err := expandSequence(cddr(expr))
		if err != nil {
			return nil, err
		}
		tmp, err := expandLambda(cons(intern("lambda"), cons(args, body)))
		if err != nil {
			return nil, err
		}
		return list(car(expr), name, tmp), nil
	} else {
		return nil, SyntaxError(expr)
	}
}

func expandLambda(expr *LList) (*LList, error) {
	exprLen := length(expr)
	if exprLen < 3 {
		return nil, SyntaxError(expr)
	}
	body, err := expandSequence(cddr(expr))
	if err != nil {
		return nil, err
	}
	bodyLen := length(body)
	if bodyLen > 0 {
		tmp := body
		if isList(tmp) && caar(tmp) == intern("define") {
			bindings := EmptyList
			for caar(tmp) == intern("define") {
				def, err := expandDefine(car(tmp).(*LList))
				if err != nil {
					return nil, err
				}
				bindings = cons(cdr(def), bindings)
				tmp = cdr(tmp)
			}
			bindings = reverse(bindings)
			tmp = cons(intern("letrec"), cons(bindings, tmp)) //scheme specifies letrec*
			tmp2, err := macroexpandList(tmp)
			return list(car(expr), cadr(expr), tmp2), err
		}
	}
	args := cadr(expr)
	return cons(car(expr), cons(args, body)), nil
}

func expandSet(expr *LList) (*LList, error) {
	exprLen := length(expr)
	if exprLen != 3 {
		return nil, SyntaxError(expr)
	}
	var val = caddr(expr)
	switch vv := val.(type) {
	case *LList:
		v, err := macroexpandList(vv)
		if err != nil {
			return nil, err
		}
		val = v
	}
	return list(car(expr), cadr(expr), val), nil
}

func expandPrimitive(fn LAny, expr *LList) (LAny, error) {
	switch fn {
	case intern("quote"):
		return expr, nil
	case intern("begin"):
		return expandSequence(expr)
	case intern("if"):
		return expandIf(expr)
	case intern("define"):
		return expandDefine(expr)
	case intern("undefine"):
		return expandUndefine(expr)
	case intern("define-macro"):
		return expandDefine(expr)
		//return expandDefineMacro(expr)
	case intern("lambda"):
		return expandLambda(expr)
	case intern("set!"):
		return expandSet(expr)
	case intern("lap"):
		return expr, nil
	case intern("use"):
		return expr, nil
	default:
		macro := getMacro(fn)
		if macro != nil {
			tmp, err := macro.expand(expr)
			return tmp, err
		}
		return nil, nil
	}
}

func crackLetrecBindings(bindings *LList, tail *LList) (*LList, *LList, bool) {
	var names []LAny
	inits := EmptyList
	for bindings != EmptyList {
		if isList(bindings) {
			tmp := car(bindings)
			if isList(tmp) {
				name := car(tmp)
				if isSymbol(name) {
					names = append(names, name)
				} else {
					return nil, nil, false
				}
				if isList(cdr(tmp)) {
					inits = cons(cons(intern("set!"), tmp.(*LList)), inits)
				} else {
					return nil, nil, false
				}
			} else {
				return nil, nil, false
			}

		} else {
			return nil, nil, false
		}
		bindings = cdr(bindings)
	}
	inits = reverse(inits)
	head := inits
	for inits.cdr != EmptyList {
		inits = inits.cdr
	}
	inits.cdr = tail
	return toList(names), head, true
}

func expandLetrec(expr LAny) (LAny, error) {
	// (letrec () expr ...) -> (begin expr ...)
	// (letrec ((x 1) (y 2)) expr ...) -> ((lambda (x y) (set! x 1) (set! y 2) expr ...) nil nil)
	body := cddr(expr)
	if body == EmptyList {
		return nil, SyntaxError(expr)
	}
	bindings := cadr(expr)
	lstBindings, ok := bindings.(*LList)
	if !ok {
		return nil, SyntaxError(expr)
	}
	names, body, ok := crackLetrecBindings(lstBindings, body)
	if !ok {
		return nil, SyntaxError(expr)
	}
	code, err := macroexpandList(cons(intern("lambda"), cons(names, body)))
	if err != nil {
		return nil, err
	}
	values := newList(length(names), Null)
	return cons(code, values), nil
}

func crackLetBindings(bindings *LList) (*LList, *LList, bool) {
	var names []LAny
	var values []LAny
	for bindings != EmptyList {
		tmp := car(bindings)
		if isList(tmp) {
			name := car(tmp)
			if isSymbol(name) {
				names = append(names, name)
				tmp2 := cdr(tmp)
				if tmp2 != EmptyList {
					val, err := macroexpandObject(car(tmp2))
					if err == nil {
						values = append(values, val)
						bindings = cdr(bindings)
						continue
					}
				}
			}
		}
		return nil, nil, false
	}
	return toList(names), toList(values), true
}

func expandLet(expr LAny) (LAny, error) {
	// (let () expr ...) -> (begin expr ...)
	// (let ((x 1) (y 2)) expr ...) -> ((lambda (x y) expr ...) 1 2)
	// (let label ((x 1) (y 2)) expr ...) -> (lambda (label) expr
	if isSymbol(cadr(expr)) {
		//return ell_expand_named_let(argv, argc)
		return expandNamedLet(expr)
	}
	bindings, ok := cadr(expr).(*LList)
	if !ok {
		return nil, SyntaxError(expr)
	}
	names, values, ok := crackLetBindings(bindings)
	if !ok {
		return nil, SyntaxError(expr)
	}
	body := cddr(expr)
	if body == EmptyList {
		return nil, SyntaxError(expr)
	}
	code, err := macroexpandList(cons(intern("lambda"), cons(names, body)))
	if err != nil {
		return nil, err
	}
	return cons(code, values), nil
}

func expandNamedLet(expr LAny) (LAny, error) {
	name := cadr(expr)
	bindings, ok := caddr(expr).(*LList)
	if !ok {
		return nil, SyntaxError(expr)
	}
	names, values, ok := crackLetBindings(bindings)
	if !ok {
		return nil, SyntaxError(expr)
	}
	body := cdddr(expr)
	tmp := list(intern("letrec"), list(list(name, cons(intern("lambda"), cons(names, body)))), cons(name, values))
	return macroexpandList(tmp)
}

func crackDoBindings(bindings *LList) (*LList, *LList, *LList, bool) {
	names := EmptyList
	inits := EmptyList
	steps := EmptyList
	for bindings != EmptyList {
		tmp := car(bindings)
		if !isList(tmp) {
			return nil, nil, nil, false
		}
		if !isSymbol(car(tmp)) {
			return nil, nil, nil, false
		}
		if !isList(cdr(tmp)) {
			return nil, nil, nil, false
		}
		names = cons(car(tmp), names)
		inits = cons(cadr(tmp), inits)
		if cddr(tmp) != EmptyList {
			steps = cons(caddr(tmp), steps)
		} else {
			steps = cons(car(tmp), steps)
		}
		bindings = cdr(bindings)
	}
	var err error
	inits2, err := macroexpandList(inits)
	if err != nil {
		return nil, nil, nil, false
	}
	inits, _ = inits2.(*LList)
	steps2, err := macroexpandList(steps)
	if err != nil {
		return nil, nil, nil, false
	}
	steps, _ = steps2.(*LList)
	return names, inits, steps, true
}

func expandDo(expr LAny) (LAny, error) {
	// (do ((myvar init-val) ...) (mytest expr ...) body ...)
	// (do ((myvar init-val step) ...) (mytest expr ...) body ...)
	var tmp LAny
	var tmpl, tmpl2 *LList
	if length(expr) < 3 {
		return nil, SyntaxError(expr)
	}

	bindings, ok := cadr(expr).(*LList)
	if !ok {
		return nil, SyntaxError(expr)
	}
	names, inits, steps, ok := crackDoBindings(bindings)
	if !ok {
		return nil, SyntaxError(expr)
	}
	tmp = caddr(expr)
	if !isList(tmp) {
		return nil, SyntaxError(expr)
	}
	tmpl = tmp.(*LList)
	exitPred := car(tmpl)
	exitExprs := LAny(Null)
	if cddr(tmpl) != EmptyList {
		exitExprs = cons(intern("begin"), cdr(tmpl))
	} else {
		exitExprs = cadr(tmpl)
	}
	loopSym := intern("system_loop")
	if cdddr(expr) != EmptyList {
		tmpl = cdddr(expr)
		tmpl = cons(intern("begin"), tmpl)
		tmpl2 = cons(loopSym, steps)
		tmpl2 = list(tmpl2)
		tmpl = cons(intern("begin"), cons(tmpl, tmpl2))
	} else {
		tmpl = cons(loopSym, steps)
	}
	tmpl = list(tmpl)
	tmpl = cons(intern("if"), cons(exitPred, cons(exitExprs, tmpl)))
	tmpl = list(intern("lambda"), names, tmpl)
	tmpl = list(loopSym, tmpl)
	tmpl = list(tmpl)
	tmpl2 = cons(loopSym, inits)
	tmpl = list(intern("letrec"), tmpl, tmpl2)
	return macroexpandList(tmpl)
}

func nextCondClause(expr LAny, clauses LAny, count int) (LAny, error) {
	var result LAny
	var err error
	tmpsym := intern("__tmp__")
	ifsym := intern("if")
	elsesym := intern("else")
	letsym := intern("let")
	begsym := intern("begin")

	clause0 := car(clauses)
	next := cdr(clauses)
	clause1 := car(next)

	if count == 2 {
		if !isList(clause1) {
			return nil, SyntaxError(expr)
		}
		if elsesym == car(clause1) {
			if cadr(clause0) == intern("=>") {
				if length(clause0) != 3 {
					return nil, SyntaxError(expr)
				}
				result = list(letsym, list(list(tmpsym, car(clause0))), list(ifsym, tmpsym, list(caddr(clause0), tmpsym), cons(begsym, cdr(clause1))))
			} else {
				result = list(ifsym, car(clause0), cons(begsym, cdr(clause0)), cons(begsym, cdr(clause1)))
			}
		} else {
			if cadr(clause1) == intern("=>") {
				if length(clause1) != 3 {
					return nil, SyntaxError(expr)
				}
				result = list(letsym, list(list(tmpsym, car(clause1))), list(ifsym, tmpsym, list(caddr(clause1), tmpsym), clause1))
			} else {
				result = list(ifsym, car(clause1), cons(begsym, cdr(clause1)))
			}
			if cadr(clause0) == intern("=>") {
				if length(clause0) != 3 {
					return nil, SyntaxError(expr)
				}
				result = list(letsym, list(list(tmpsym, car(clause0))), list(ifsym, tmpsym, list(caddr(clause0), tmpsym), result))
			} else {
				result = list(ifsym, car(clause0), cons(begsym, cdr(clause0)), result)
			}
		}
	} else {
		result, err = nextCondClause(expr, next, count-1)
		if err != nil {
			return nil, err
		}
		if cadr(clause0) == intern("=>") {
			if length(clause0) != 3 {
				return nil, SyntaxError(expr)
			}
			result = list(letsym, list(list(tmpsym, car(clause0))), list(ifsym, tmpsym, list(caddr(clause0), tmpsym), result))
		} else {
			result = list(ifsym, car(clause0), cons(begsym, cdr(clause0)), result)
		}
	}
	return macroexpand(result)
}

func expandCond(expr LAny) (LAny, error) {
	i := length(expr)
	if i < 2 {
		return nil, SyntaxError(expr)
	} else if i == 2 {
		tmp := cadr(expr)
		if car(tmp) == intern("else") {
			tmp = cons(intern("begin"), cdr(tmp))
		} else {
			expr = cons(intern("begin"), cdr(tmp))
			tmp = list(intern("if"), car(tmp), expr)
		}
		return macroexpand(tmp)
	} else {
		return nextCondClause(expr, cdr(expr), i-1)
	}
}

func expandQuasiquote(expr LAny) (LAny, error) {
	if length(expr) != 2 {
		return nil, SyntaxError(expr)
	}
	return expandQQ(cadr(expr))
}

func expandQQ(expr LAny) (LAny, error) {
	switch v := expr.(type) {
	case *LList:
		if v == EmptyList {
			return v, nil
		}
		if v.cdr != EmptyList {
			if v.car == symUnquote {
				if v.cdr.cdr != EmptyList {
					return nil, SyntaxError(v)
				}
				return macroexpand(v.cdr.car)
			} else if v.car == symUnquoteSplicing {
				return nil, Error("unquote-splicing can only occur in the context of a list ")
			}
		}
		tmp, err := expandQQList(v)
		if err != nil {
			return nil, err
		}
		return macroexpand(tmp)
	case *LSymbol:
		if isKeyword(v) {
			return expr, nil
		}
		return list(intern("quote"), expr), nil
	default: //all other objects evaluate to themselves
		return expr, nil
	}
}

func expandQQList(lst *LList) (LAny, error) {
	var tmp LAny
	var err error
	result := list(intern("concat"))
	tail := result
	for lst != EmptyList {
		item := car(lst)
		if isList(item) && item != EmptyList {
			if car(item) == symQuasiquote {
				return nil, Error("nested quasiquote not supported")
			}
			if car(item) == symUnquote && length(item) == 2 {
				tmp, err = macroexpand(cadr(item))
				tmp = list(intern("list"), tmp)
				if err != nil {
					return nil, err
				}
				tail.cdr = list(tmp)
				tail = tail.cdr
			} else if car(item) == symUnquoteSplicing && length(item) == 2 {
				tmp, err = macroexpand(cadr(item))
				if err != nil {
					return nil, err
				}
				tail.cdr = list(tmp)
				tail = tail.cdr
			} else {
				tmp, err = expandQQList(item.(*LList))
				if err != nil {
					return nil, err
				}
				tail.cdr = list(list(intern("list"), tmp))
				tail = tail.cdr
			}
		} else {
			tail.cdr = list(list(intern("quote"), list(item)))
			tail = tail.cdr
		}
		lst = cdr(lst)
	}
	return result, nil
}
