// Copyright 2017 Dave Astels.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package implements a basic LISP interpretor for embedding in a go program for scripting.
// This file contains a new interpretor based on that in Principles of AI programming.

package golisp

import (
	"fmt"
	"sync/atomic"
)

// Interned symbols for interp branching

var quoteSym = Intern("quote")
var beginSym = Intern("begin")
var setSym = Intern("set!")
var ifSym = Intern("if")
var lambdaSym = Intern("lambda")
var namedLambdaSym = Intern("named-lambda")

// interned symbols for use in desugaring

var getslotSym = Intern("get-slot")
var setslotSym = Intern("set-slot!")
var hasslotSym = Intern("has-slot?")
var sendSym = Intern("send")
var sendsuperSym = Intern("send-super")
var channelreadSym = Intern("channel-read")
var channelwriteSym = Intern("channel-write")

func printDashes(indent int) {
	for i := indent; i > 0; i -= 1 {
		fmt.Print("-")
	}
}

func logEval(d *Data, env *SymbolTableFrame) {
	if LispTrace && !DebugEvalInDebugRepl {
		depth := env.Depth()
		fmt.Printf("%3d: ", depth)
		printDashes(depth)
		fmt.Printf("> %s\n", String(d))
		EvalDepth += 1
	}
}

func logResult(result *Data, env *SymbolTableFrame) {
	if LispTrace && !DebugEvalInDebugRepl {
		depth := env.Depth()
		fmt.Printf("%3d: <", depth)
		printDashes(depth)
		fmt.Printf(" %s\n", String(result))
	}
}

func postProcessShortcuts(d *Data) *Data {
	symbolObj := Car(d)

	if !SymbolP(symbolObj) {
		return d
	}

	pseudoFunction := StringValue(symbolObj)

	stringLength := len(pseudoFunction)

	if stringLength < 2 {
		return d
	} else if pseudoFunction[stringLength-1] == ':' {
		return AppendBangList(InternalMakeList(getslotSym, Cadr(d), symbolObj), Cddr(d))
	} else if pseudoFunction[stringLength-2] == ':' {
		if pseudoFunction[stringLength-1] == '!' {
			return AppendBangList(InternalMakeList(setslotSym, Cadr(d), Intern(pseudoFunction[0:stringLength-1]), Caddr(d)), Cdddr(d))
		} else if pseudoFunction[stringLength-1] == '?' {
			return AppendBangList(InternalMakeList(hasslotSym, Cadr(d), Intern(pseudoFunction[0:stringLength-1])), Cddr(d))
		} else if pseudoFunction[stringLength-1] == '>' {
			return AppendBangList(InternalMakeList(sendSym, Cadr(d), Intern(pseudoFunction[0:stringLength-1])), Cddr(d))
		} else if pseudoFunction[stringLength-1] == '^' {
			return AppendBangList(InternalMakeList(sendsuperSym, Intern(pseudoFunction[0:stringLength-1])), Cdr(d))
		}
	} else if pseudoFunction[0] == '<' && pseudoFunction[1] == '-' {
		return AppendBangList(InternalMakeList(channelreadSym, Intern(pseudoFunction[2:stringLength])), Cdr(d))
	} else if pseudoFunction[stringLength-2] == '<' && pseudoFunction[stringLength-1] == '-' {
		return AppendBangList(InternalMakeList(channelwriteSym, Intern(pseudoFunction[0:stringLength-2])), Cdr(d))
	}
	return d
}

func evalHelper(x *Data, env *SymbolTableFrame, needFunction bool) (result *Data, err error) {
	//	fmt.Printf("############ ENTERING EVAL ###########\n")
INTERP:
	logEval(x, env)

	if NilP(x) {
		result = nil
	} else if x.Type == SymbolType {
		//		fmt.Printf("Symbol: %s\n", String(x))
		s := StringValue(x)
		if s[len(s)-1] == ':' {
			result = x
		} else {
			result = env.ValueOfWithFunctionSlotCheck(x, needFunction)
		}
	} else if (x.Type & AtomType) != 0 {
		//		fmt.Printf("Atom: %s\n", String(x))
		result = x
	} else { // list
		x = postProcessShortcuts(x)
		head := Car(x)

		if head == quoteSym {
			//			fmt.Printf("quote: %s\n", String(Cadr(x)))
			result = Cadr(x)
		} else if head == beginSym {
			//			fmt.Printf("begin\n")
			var cell *Data
			for cell = Cdr(x); NotNilP(Cdr(cell)); cell = Cdr(cell) {
				result, err = Eval(Car(cell), env)
				if err != nil {
					return
				}
			}
			x = Car(cell)
			goto INTERP
		} else if head == setSym {
			//			fmt.Printf("set!: %s\n", String(Cadr(x)))
			var v *Data
			v, err = Eval(Caddr(x), env)
			if err != nil {
				return nil, err
			}
			result, err = env.BindTo(Cadr(x), v)
			if err != nil {
				return
			}
		} else if head == ifSym {
			//			fmt.Printf("if\n")
			var c *Data
			c, err = Eval(Second(x), env)
			if err != nil {
				return nil, err
			}
			if BooleanValue(c) {
				x = Third(x)
			} else {
				x = Fourth(x)
			}
			goto INTERP
		} else if head == lambdaSym {
			//			fmt.Printf("lambda\n")
			formals := Cadr(x)
			if !ListP(formals) && !DottedListP(formals) {
				err = ProcessError(fmt.Sprintf("lambda requires a parameter list but recieved %s.", String(formals)), env)
				return
			}
			params := formals
			body := Cddr(x)
			return FunctionWithNameParamsDocBodyAndParent("unnamed", params, "", body, env), nil
		} else if head == namedLambdaSym {
			//			fmt.Printf("named-lambda\n")
			formals := Cadr(x)
			if !ListP(formals) && !DottedListP(formals) {
				err = ProcessError(fmt.Sprintf("named-lambda requires a formals list but recieved %s.", String(formals)), env)
				return
			}
			name := Car(formals)
			if !SymbolP(name) {
				err = ProcessError(fmt.Sprintf("named-lambda requires a Symbol name but recieved %s.", String(name)), env)
				return
			}
			params := Cdr(formals)
			body := Cddr(x)
			return FunctionWithNameParamsDocBodyAndParent(StringValue(name), params, "", body, env), nil
		} else {
			//			fmt.Printf("expression: %s\n", String(x))

			proc, err := evalHelper(Car(x), env, true)
			if err != nil {
				return nil, err
			}
			var argList *Data
			if FunctionP(proc) || (PrimitiveP(proc) && !PrimitiveValue(proc).Special) {
				args := make([]*Data, 0, Length(x)-1)
				for cell := Cdr(x); NotNilP(cell); cell = Cdr(cell) {
					var v *Data
					v, err = Eval(Car(cell), env)
					if err != nil {
						return nil, err
					}
					// if PrimitiveP(proc) && PrimitiveValue(proc).Name == "+" {
					// 	fmt.Printf("Adding arg %s from %s\n", String(v), String(Car(cell)))
					// }

					args = append(args, v)
				}
				argList = ArrayToList(args)
			} else {
				argList = Cdr(x)
			}
			// if PrimitiveP(proc) && PrimitiveValue(proc).Name == "+" {
			// 	fmt.Printf("Args: %s\n", String(argList))
			// }
			if MacroP(proc) {
				//				fmt.Printf("macro: %s\n", String(proc))
				x, err = MacroValue(proc).Expand(Cdr(x), env)
				if err != nil {
					return nil, err
				}
				goto INTERP
			} else if FunctionP(proc) {
				//				fmt.Printf("function: %s\n", String(proc))
				f := FunctionValue(proc)
				var fr *FrameMap
				if atomic.LoadInt32(&f.SlotFunction) == 1 && env.HasFrame() {
					fr = env.Frame
				}
				env, err = f.ExtendEnv(argList, env, fr)
				if err != nil {
					return nil, err
				}
				x = Cons(Intern("begin"), f.Body)
				goto INTERP
			} else {
				//				fmt.Printf("primitive: %s\n", String(proc))
				result, err = ApplyWithoutEval(proc, argList, env)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	logResult(result, env)
	return
}

func Eval(d *Data, env *SymbolTableFrame) (result *Data, err error) {
	return evalHelper(d, env, false)
}