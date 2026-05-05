package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const systemHost = "http://localhost:8080"

func main() {
	input, err := parseInput()
	if err != nil {
		panic(err)
	}

	switch input.Action {
	case ShowAction:
		records, err := getRecords(input.RequestContext)
		if err != nil {
			panic(err)
		}
		request := getRequest(input.RequestContext, records)

		printRequest(request, request.Out.StatusCode, 0)
	case ReplayAction:
		if len(input.Mapping) == 0 {
			fmt.Println("No service mapping to replay")
			return
		}

		records, err := getRecords(input.RequestContext)
		if err != nil {
			panic(err)
		}
		addDebugData(input.RequestContext, records)
		closer, err := startDebugHost()
		defer closer()
		if err != nil {
			panic(err)
		}
		request := getRequest(input.RequestContext, records)

		count := replayRequest(request, input.Mapping, 0)
		if count == 0 {
			fmt.Println("No replayable service mapping")
		}
	case RegressAction:
		if len(input.Mapping) == 0 {
			fmt.Println("No service mapping to replay")
			return
		}

		serviceNames := []string{}
		for k := range input.Mapping {
			serviceNames = append(serviceNames, k)
		}

		list, err := getList(serviceNames, []string{input.RequestContext}, input.ValidStatus, input.MaxCount)
		if err != nil {
			panic(err)
		}

		regressFailCount := 0

		for _, rc := range list {
			records, err := getRecords(rc)
			if err != nil {
				fmt.Printf("Error regressing request: %s\n", err.Error())
				regressFailCount++
				continue
			}

			addDebugData(rc, records)
		}

		closer, err := startDebugHost()
		defer closer()
		if err != nil {
			panic(err)
		}

		for i, rc := range list {
			records := getDebugRecords(rc)

			if records == nil {
				continue // Previously fail counted
			}

			request := getRequest(rc, records)

			fmt.Printf("Regressing request (%d) %s\n", i+1, rc)
			_, passed := regressRequest(request, input.Mapping, 0)
			if !passed {
				fmt.Println("Regression failed")
				regressFailCount++
			}
		}

		if regressFailCount > 0 {
			fmt.Printf("Regression Test Failed: %d out of %d requests\n", regressFailCount, len(list))
		} else {
			fmt.Printf("Regression Test Passed! (%d/%d)\n", len(list), len(list))
		}

	default:
		fmt.Println("Unknown action")
	}
}

func getList(serviceNames []string, excludedContexts []string, statusCodes []int, max int) ([]string, error) {
	statusCodesStr := []string{}
	for _, c := range statusCodes {
		statusCodesStr = append(statusCodesStr, strconv.Itoa(c))
	}
	listUri := systemHost + "/list?sr=" + strings.Join(serviceNames, "|") + "&ex=" + strings.Join(excludedContexts, "|") + "&st=" + strings.Join(statusCodesStr, "|") + "&n=" + strconv.Itoa(max)

	resp, err := http.Get(listUri)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println(listUri)
		return []string{}, fmt.Errorf("Please check input")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{}, err
	}

	var list []string

	if err = json.Unmarshal(data, &list); err != nil {
		return []string{}, err
	}
	return list, nil
}

func getRequest(rc string, records []Record) Request {
	ec := ""
	// Find the initial request
	for _, r := range records {
		// Records of inital request has cause context as request context
		if r.RequestContext == rc && r.CauseContext == rc {
			ec = r.ExecutionContext
			break
		}
	}

	return buildRequestTree(records, ec)
}

func getRecords(rc string) ([]Record, error) {
	getUri := systemHost + "/get?rc=" + rc

	resp, err := http.Get(getUri)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Coudn't find request %s", rc)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	records := []Record{}

	if err = json.Unmarshal(data, &records); err != nil {
		return nil, err
	}

	return records, nil
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
	fmt.Printf("%s-> %s (%d)\n", pre, serviceName, statusCode)

	obPre := getPreposition(level + 1)
	for i := range request.Observations {
		fmt.Printf("%s-> Internal <%s[%d]>\n", obPre, request.Observations[i].ObservationName, request.Observations[i].ScopedSequence)
	}

	for i := range request.Dependencies {
		printRequest(request.Dependencies[i].Reference, request.Dependencies[i].Out.StatusCode, level+1)
	}
}

func parseInput() (Input, error) {
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

func replayRequest(request Request, mapping map[string]string, count int) int {
	serviceKey := strings.ToLower(request.In.ServiceName)

	if host, ok := mapping[serviceKey]; ok {
		count++
		in := request.In
		url := "http://" + host + in.Uri
		var body io.Reader

		if len(in.Body) > 0 {
			body = bytes.NewBuffer(in.Body)
		}

		httpRquest, err := http.NewRequest(in.Method, url, body)
		if err != nil {
			fmt.Printf("request error %s\n", err.Error())
			return count
		}

		fmt.Println(url)

		httpRquest.Header = in.Header
		httpRquest.Header.Set(RequestContextHeader, in.RequestContext)
		httpRquest.Header.Set(CauseContextHeader, in.CauseContext)
		httpRquest.Header.Set(ExecutionContextHeader, in.ExecutionContext)
		httpRquest.Header.Set(ServiceDebugHeader, DebugEnabled)
		httpRquest.Header.Set(DebugConfigHeader, debugConfig(mapping))
		httpRquest.Header.Set(DebugHostHeader, DebugHost)

		resp, err := http.DefaultClient.Do(httpRquest)
		if err != nil {
			fmt.Printf("response error %s\n", err.Error())
			return count
		}

		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("read error %s\n", err.Error())
			return count
		}

		fmt.Printf("Body: %s", string(respBody))
	} else {
		// Only replay dependencies if the request itself isn't replayed
		for _, dep := range request.Dependencies {
			if dep.Reference.In.ServiceName != "" {
				count += replayRequest(dep.Reference, mapping, count)
			}
		}
	}
	return count
}

func regressRequest(request Request, mapping map[string]string, count int) (int, bool) {
	serviceKey := strings.ToLower(request.In.ServiceName)

	requestRegPassed := true

	if host, ok := mapping[serviceKey]; ok {
		count++
		in := request.In
		url := "http://" + host + in.Uri
		var body io.Reader

		if len(in.Body) > 0 {
			body = bytes.NewBuffer(in.Body)
		}

		httpRquest, err := http.NewRequest(in.Method, url, body)
		if err != nil {
			fmt.Printf("request error %s\n", err.Error())
			return count, false
		}

		fmt.Println(url)

		httpRquest.Header = in.Header
		httpRquest.Header.Set(RequestContextHeader, in.RequestContext)
		httpRquest.Header.Set(CauseContextHeader, in.CauseContext)
		httpRquest.Header.Set(ExecutionContextHeader, in.ExecutionContext)
		httpRquest.Header.Set(ServiceDebugHeader, DebugEnabled)
		httpRquest.Header.Set(DebugConfigHeader, debugConfig(mapping))
		httpRquest.Header.Set(DebugHostHeader, DebugHost)

		resp, err := http.DefaultClient.Do(httpRquest)
		if err != nil {
			fmt.Printf("response error %s\n", err.Error())
			return count, false
		}

		fmt.Printf("Response Status: %d\n", resp.StatusCode)
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("read error %s\n", err.Error())
			return count, false
		}

		requestRegPassed = requestRegPassed && bytes.Equal(respBody, request.Out.Body)

		fmt.Printf("Body: %s\n", string(respBody))
	} else {
		// Only regress dependencies if the request itself isn't regressed
		for _, dep := range request.Dependencies {
			if dep.Reference.In.ServiceName != "" {
				regCount, regPassed := regressRequest(dep.Reference, mapping, count)
				count += regCount
				requestRegPassed = requestRegPassed && regPassed
			}
		}
	}

	return count, requestRegPassed
}
