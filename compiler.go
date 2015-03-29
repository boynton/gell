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

func compile(module module, expr lob) (code, error) {
	code := newCode(module, 0, nil, nil, "")
	err := compileExpr(code, NIL, expr, false, false, "")
	if err != nil {
		return nil, err
	}
	code.emitReturn()
	return code, nil
}

func calculateLocation(sym lob, env lob) (int, int, bool) {
	i := 0
	for isPair(env) {
		j := 0
		e := car(env)
		for isPair(e) {
			if car(e) == sym {
				return i, j, true
			}
			j++
			e = cdr(e)
		}
		i++
		env = cdr(env)
	}
	return -1, -1, false
}

func compileExpr(code code, env lob, expr lob, isTail bool, ignoreResult bool, context string) error {
	//Println("COMPILE: ", expr, " isTail: ", isTail, ", ignoreResult: ", ignoreResult)
	if isSymbol(expr) {
		if i, j, ok := calculateLocation(expr, env); ok {
			code.emitLocal(i, j)
		} else {
			code.emitGlobal(expr)
		}
		if ignoreResult {
			code.emitPop()
		} else if isTail {
			code.emitReturn()
		}
		return nil
	} else if isPair(expr) {
		lst := expr
		lstlen := length(lst)
		if lstlen == 0 {
			return syntaxError(lst)
		}
		fn := car(lst)
		switch fn {
		case intern("quote"):
			// (quote <datum>)
			if lstlen != 2 {
				return syntaxError(expr)
			}
			if !ignoreResult {
				code.emitLiteral(cadr(lst))
				if isTail {
					code.emitReturn()
				}
			}
			return nil
		case intern("begin"):
			// (begin <expr> ...)
			return compileSequence(code, env, cdr(lst), isTail, ignoreResult, context)
		case intern("if"):
			// (if <pred> <consequent>)
			// (if <pred> <consequent> <antecedent>)
			if lstlen == 3 || lstlen == 4 {
				return compileIfElse(code, env, cadr(expr), caddr(expr), cdddr(expr), isTail, ignoreResult, context)
			}
			return syntaxError(expr)
		case intern("define"):
			// (define <name> <val>)
			if lstlen < 3 {
				return syntaxError(expr)
			}
			sym := cadr(lst)
			val := caddr(lst)
			if !isSymbol(sym) {
				if isPair(sym) && length(sym) >= 1 {
					args := cdr(sym)
					sym = car(sym)
					//we could give the symbolic name to the function
					val = list(intern("lambda"), args, val)
				} else {
					return syntaxError(expr)
				}
			}
			err := compileExpr(code, env, val, false, false, sym.String())
			if err == nil {
				code.emitDefGlobal(sym)
				if ignoreResult {
					code.emitPop()
				} else if isTail {
					code.emitReturn()
				}
			}
			return err
		case intern("define-macro"):
			// (defmacro <name> (lambda (expr) '(the expanded value)))
			if lstlen != 3 {
				return syntaxError(expr)
			}
			var sym = cadr(lst)
			if !isSymbol(sym) {
				return syntaxError(expr)
			}
			err := compileExpr(code, env, caddr(lst), false, false, sym.String())
			if err == nil {
				code.emitDefMacro(sym)
				if ignoreResult {
					code.emitPop()
				} else if isTail {
					code.emitReturn()
				}
			}
			return err
		case intern("lambda"):
			// (lambda ()  <expr> ...)
			// (lambda (sym ...)  <expr> ...)
			// (lambda (sym ... . rest)  <expr> ...)
			// (lambda sym <expr> ...) ;; all args in a list, bound to sym
			if lstlen < 3 {
				return syntaxError(expr)
			}
			body := cddr(lst)
			args := cadr(lst)
			return compileLambda(code, env, args, body, isTail, ignoreResult, context)
		case intern("set!"):
			// (set! <sym> <val>)
			if lstlen != 3 {
				return syntaxError(expr)
			}
			var sym = cadr(lst)
			if !isSymbol(sym) {
				return syntaxError(expr)
			}
			err := compileExpr(code, env, caddr(lst), false, false, context)
			if err != nil {
				return err
			}
			if i, j, ok := calculateLocation(sym, env); ok {
				code.emitSetLocal(i, j)
			} else {
				code.emitDefGlobal(sym) //fix, should be SetGlobal!!!
			}
			if ignoreResult {
				code.emitPop()
			} else if isTail {
				code.emitReturn()
			}
			return nil
		case intern("lap"):
			// (lap <instruction> ...)
			return code.loadOps(cdr(expr))
		case intern("use"):
			// (use module_name)
			return compileUse(code, cdr(lst))
		default: // a funcall
			// (<fn>)
			// (<fn> <arg> ...)
			return compileFuncall(code, env, fn, cdr(lst), isTail, ignoreResult, context)
		}
	} else if vec, ok := expr.(*lvector); ok {
		//vector literal: the elements are evaluated
		vlen := len(vec.elements)
		for i := vlen - 1; i >= 0; i-- {
			obj := vec.elements[i]
			err := compileExpr(code, env, obj, false, false, context)
			if err != nil {
				return err
			}
		}
		code.emitVector(vlen)
		return nil
	} else if amap, ok := expr.(*lmap); ok {
		//vector literal: the elements are evaluated
		mlen := len(amap.bindings)
		vlen := mlen * 2
		vals := make([]lob, 0, vlen)
		for k, v := range amap.bindings {
			vals = append(vals, k)
			vals = append(vals, v)
		}
		for i := vlen - 1; i >= 0; i-- {
			obj := vals[i]
			err := compileExpr(code, env, obj, false, false, context)
			if err != nil {
				return err
			}
		}
		code.emitMap(vlen)
		return nil
	} else {
		if !ignoreResult {
			code.emitLiteral(expr)
			if isTail {
				code.emitReturn()
			}
		}
		return nil
	}
}

func compileLambda(code code, env lob, args lob, body lob, isTail bool, ignoreResult bool, context string) error {
	argc := 0
	syms := []lob{}
	var defaults []lob
	var keys []lob
	tmp := args
	//to do: deal with rest, optional, and keywords arguments
	for isPair(tmp) {
		a := car(tmp)
		if vec, ok := a.(*lvector); ok {
			//i.e. (x [y (z 23)]) is for optional y and z, but bound, z with default 23
			if cdr(tmp) != NIL {
				return syntaxError(tmp)
			}
			defaults = make([]lob, 0, len(vec.elements))
			for _, sym := range vec.elements {
				var def lob = NIL
				if lst, ok := sym.(*lpair); ok {
					next := lst.cdr
					sym = lst.car
					if lst2, ok := next.(*lpair); ok {
						def = lst2.car
					}
				}
				if !isSymbol(sym) {
					return syntaxError(tmp)
				}
				syms = append(syms, sym)
				defaults = append(defaults, def)
			}
			tmp = NIL
			break
		} else if mp, ok := a.(*lmap); ok {
			//i.e. (x {y: 23, z: 57}]) is for optional y and z, keyword args, with defaults
			if cdr(tmp) != NIL {
				return syntaxError(tmp)
			}
			defaults = make([]lob, 0, len(mp.bindings))
			keys = make([]lob, 0, len(mp.bindings))
			for sym, defValue := range mp.bindings {
				if isPair(sym) && car(sym) == intern("quote") && isPair(cdr(sym)) {
					sym = cadr(sym)
				}
				if !isSymbol(sym) {
					return syntaxError(tmp)
				}
				syms = append(syms, sym)
				keys = append(keys, sym)
				defaults = append(defaults, defValue)
			}
			tmp = NIL
			break
		} else if !isSymbol(a) {
			return syntaxError(tmp)
		}
		argc++
		syms = append(syms, a)
		tmp = cdr(tmp)
	}
	if tmp != NIL {
		//rest arg
		if isSymbol(tmp) {
			syms = append(syms, tmp) //note: added, but argv not incremented
			defaults = make([]lob, 0)
		} else {
			return syntaxError(tmp)
		}
	}
	args = toList(syms) //why not just use the array format in general?
	newEnv := cons(args, env)
	mod := (code.(*lcode)).module()
	lambdaCode := newCode(mod, argc, defaults, keys, context)
	err := compileSequence(lambdaCode, newEnv, body, true, false, context)
	if err == nil {
		if !ignoreResult {
			code.emitClosure(lambdaCode)
			if isTail {
				code.emitReturn()
			}
		}
	}
	return err
}

func compileSequence(code code, env lob, exprs lob, isTail bool, ignoreResult bool, context string) error {
	if exprs != NIL {
		for cdr(exprs) != NIL {
			err := compileExpr(code, env, car(exprs), false, true, context)
			if err != nil {
				return err
			}
			exprs = cdr(exprs)
		}
		return compileExpr(code, env, car(exprs), isTail, ignoreResult, context)
	}
	return syntaxError(cons(intern("begin"), exprs))
}

func compileFuncall(code code, env lob, fn lob, args lob, isTail bool, ignoreResult bool, context string) error {
	argc := length(args)
	if argc < 0 {
		return syntaxError(cons(fn, args))
	}
	err := compileArgs(code, env, args, context)
	if err != nil {
		return err
	}
	if extendedInstructions {
		ok, err := compilePrimopCall(code, fn, argc, isTail, ignoreResult)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	err = compileExpr(code, env, fn, false, false, context)
	if err != nil {
		return err
	}
	if isTail {
		code.emitTailCall(argc)
	} else {
		code.emitCall(argc)
		if ignoreResult {
			code.emitPop()
		}
	}
	return nil
}

func compileArgs(code code, env lob, args lob, context string) error {
	if args != NIL {
		err := compileArgs(code, env, cdr(args), context)
		if err != nil {
			return err
		}
		return compileExpr(code, env, car(args), false, false, context)
	}
	return nil
}

func compilePrimopCall(code code, fn lob, argc int, isTail bool, ignoreResult bool) (bool, error) {
	switch fn {
	case intern("car"):
		if argc != 1 {
			return false, nil
		}
		code.emitCar()
	case intern("cdr"):
		if argc != 1 {
			return false, nil
		}
		code.emitCdr()
	case intern("null?"):
		if argc != 1 {
			return false, nil
		}
		code.emitNull()
	case intern("+"):
		if argc != 2 {
			return false, nil
		}
		code.emitAdd()
	case intern("*"):
		if argc != 2 {
			return false, nil
		}
		code.emitMul()
	default:
		return false, nil
	}
	if isTail {
		code.emitReturn()
	} else if ignoreResult {
		code.emitPop()
	}
	return true, nil
}

func compileIfElse(code code, env lob, predicate lob, consequent lob, antecedentOptional lob, isTail bool, ignoreResult bool, context string) error {
	var antecedent lob = NIL
	if antecedentOptional != NIL {
		antecedent = car(antecedentOptional)
	}
	err := compileExpr(code, env, predicate, false, false, context)
	if err != nil {
		return err
	}
	loc1 := code.emitJumpFalse(0) //returns the location just *after* the jump. setJumpLocation knows this.
	err = compileExpr(code, env, consequent, isTail, ignoreResult, context)
	if err != nil {
		return err
	}
	loc2 := 0
	if !isTail {
		loc2 = code.emitJump(0)
	}
	code.setJumpLocation(loc1)
	err = compileExpr(code, env, antecedent, isTail, ignoreResult, context)
	if err == nil {
		if !isTail {
			code.setJumpLocation(loc2)
		}
	}
	return err
}

func compileUse(code code, rest lob) error {
	lstlen := length(rest)
	if lstlen != 1 {
		//to do: other options for use.
		return syntaxError(cons(intern("use"), rest))
	}
	sym := car(rest)
	if !isSymbol(sym) {
		return syntaxError(rest)
	}
	code.emitUse(sym)
	return nil
}

func syntaxError(expr lob) error {
	return newError(expr)
}
