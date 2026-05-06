package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ParseInput() (Input, error) {
	args := os.Args[1:]

	i := Input{
		Mapping:     map[string]string{},
		ValidStatus: []int{200},
		MaxCount:    10,
	}

	if len(args) < 2 {
		return i, fmt.Errorf("Not enough arguments")
	}

	i.Action = Action(args[0])

	args = args[1:]
	ctx := parsingContextRequest // first argument is expected to be request context unless otherwise specified

	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		ctx = parseNextArg(&i, arg, ctx)
	}

	return i, nil
}

type parsingContext string

const (
	parsingContextNone              = parsingContext("NONE")
	parsingContextRequest           = parsingContext("--request")
	parsingContextMap               = parsingContext("--map")
	parsingContextCount             = parsingContext("--count")
	parsingContextRegressDependency = parsingContext("--regress-dependency")
	parsingContextAllowDiversion    = parsingContext("--allow-diversion")

	noFlagDiversion = "no-flag"
)

func parseNextArg(i *Input, arg string, ctx parsingContext) parsingContext {
	switch arg {
	case string(parsingContextRequest):
		return parsingContextRequest
	case string(parsingContextMap):
		return parsingContextMap
	case string(parsingContextCount):
		return parsingContextCount
	case string(parsingContextRegressDependency):
		i.RegressDependency = true
		return parsingContextNone
	case string(parsingContextAllowDiversion):
		i.AllowDiversion = true
		return parsingContextAllowDiversion
	}

	switch ctx {
	case parsingContextRequest:
		i.RequestContext = append(i.RequestContext, arg)
		return parsingContextRequest
	case parsingContextMap:
		sh := strings.Split(arg, "=")
		if len(sh) == 2 {
			i.Mapping[strings.ToLower(sh[0])] = sh[1]
		}
		return ctx
	case parsingContextCount:
		count, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Println("--count must be a number")
		} else {
			i.MaxCount = count
		}
		return parsingContextNone
	case parsingContextAllowDiversion:
		if arg == noFlagDiversion {
			i.NoFlagDiversion = true
		}
		return parsingContextNone
	default:
		return parsingContextNone
	}
}
