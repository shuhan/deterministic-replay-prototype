package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const debugHost = "http://localhost:8080/runtime/replay?rc="

func main() {
	input, err := parseInput()
	if err != nil {
		panic(err)
	}

	switch input.Action {
	case ShowAction:
		request, err := getRequest(input.RequestContext)
		if err != nil {
			panic(err)
		}
		printRequest(request, request.Out.StatusCode, 0)
		break
	case ReplayAction:
		if len(input.Mapping) == 0 {
			fmt.Println("No service mapping to replay")
			return
		}

		request, err := getRequest(input.RequestContext)
		if err != nil {
			panic(err)
		}

		replayRequest(request, input.Mapping)
		break
	default:
		fmt.Println("Unknown action")
	}
}

func getRequest(rc string) (Request, error) {
	replayUri := debugHost + rc

	resp, err := http.Get(replayUri)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		return Request{}, fmt.Errorf("Coudn't find request %s", rc)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return Request{}, err
	}

	request := Request{}

	if err = json.Unmarshal(data, &request); err != nil {
		return Request{}, err
	}
	return request, nil
}

func getPreposition(level int) string {
	return strings.Join(make([]string, level+1), "    ")
}

func printRequest(request Request, statusCode, level int) {
	pre := getPreposition(level)
	serviceName := request.Out.ServiceName
	if serviceName == "" {
		serviceName = "[External]"
	}
	fmt.Printf("%s->%s (%d)\n", pre, serviceName, statusCode)

	for i := range request.Dependencies {
		printRequest(request.Dependencies[i].Reference, request.Dependencies[i].Out.StatusCode, level+1)
	}
}

func parseInput() (Input, error) {
	args := os.Args[1:]

	i := Input{
		Mapping: map[string]string{},
	}

	if len(args) < 2 {
		return i, fmt.Errorf("Not enough arguments")
	}

	i.Action = Action(args[0])
	i.RequestContext = args[1]

	args = args[2:]

	if len(args) > 0 && args[0] == "--map" {
		args = args[1:]
		for len(args) > 0 {
			arg := args[0]
			args = args[1:]
			sh := strings.Split(arg, "=")
			if len(sh) == 2 {
				i.Mapping[strings.ToLower(sh[0])] = sh[1]
			}
		}
	}
	return i, nil
}

func replayRequest(request Request, mapping map[string]string) {
	serviceKey := strings.ToLower(request.In.ServiceName)

	if host, ok := mapping[serviceKey]; ok {
		in := request.In
		url := "http://" + host + in.Uri
		var body io.Reader

		if in.Body != nil && len(in.Body) > 0 {
			body = bytes.NewBuffer(in.Body)
		}

		httpRquest, err := http.NewRequest(in.Method, url, body)
		if err != nil {
			fmt.Printf("request error %s\n", err.Error())
			return
		}

		fmt.Println(url)

		httpRquest.Header = in.Header
		httpRquest.Header.Set(RequestContextHeader, in.RequestContext)
		httpRquest.Header.Set(CauseContextHeader, in.CauseContext)
		httpRquest.Header.Set(ExecutionContextHeader, in.ExecutionContext)
		httpRquest.Header.Set(ServiceDebugHeader, DebugEnabled)
		httpRquest.Header.Set(DebugConfigHeader, debugConfig(mapping))

		resp, err := http.DefaultClient.Do(httpRquest)
		if err != nil {
			fmt.Printf("response error %s\n", err.Error())
			return
		}

		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("read error %s\n", err.Error())
			return
		}

		fmt.Printf("Body: %s", string(respBody))
	} else {
		// Only replay dependencies if the request itself isn't replayed
		for _, dep := range request.Dependencies {
			if dep.Reference.In.ServiceName != "" {
				replayRequest(dep.Reference, mapping)
			}
		}
	}
}

func debugConfig(mapping map[string]string) string {
	retval := ""

	for k, v := range mapping {
		if retval != "" {
			retval += "|"
		}
		retval += k + "=" + v
	}

	return retval
}
