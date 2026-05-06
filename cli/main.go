package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const systemHost = "http://localhost:8080"

func main() {
	input, err := ParseInput()
	if err != nil {
		panic(err)
	}

	switch input.Action {
	case ShowAction:
		records, err := GetRecords(input.RequestContext)
		if err != nil {
			fmt.Println(err)
			return
		}
		request := BuildRequest(input.RequestContext, records)

		PrintRequest(request, request.Out.StatusCode, 0)
	case ReplayAction:
		if len(input.Mapping) == 0 {
			fmt.Println("No service mapping to replay")
			return
		}

		records, err := GetRecords(input.RequestContext)
		if err != nil {
			fmt.Println(err)
			return
		}
		request := BuildRequest(input.RequestContext, records)

		closer, err := StartDebugHost(input.AllowDiversion)
		defer closer()
		if err != nil {
			fmt.Println(err)
			return
		}
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
			fmt.Println(err)
			return
		}

		regressFailCount := 0

		closer, err := StartDebugHost(input.AllowDiversion)
		defer closer()
		if err != nil {
			fmt.Println(err)
			return
		}
		for i, rc := range list {
			fmt.Printf("Regressing request (%d) %s\n", i+1, rc)
			records, err := GetRecords(rc)
			if err != nil {
				fmt.Printf("Error regressing request: %s\n", err.Error())
				regressFailCount++
				continue
			}

			request := BuildRequest(rc, records)
			ResetDependencyRegression()
			_, passed := regressRequest(request, input.Mapping, 0)
			if !passed {
				fmt.Println("Regression failed")
				regressFailCount++
			} else {
				if input.RegressDependency && !hasDependencyRegressionPassed() {
					fmt.Println("Dependency Regression Failed")
					regressFailCount++
				}
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
		httpRquest.Header.Set(DebugConfigHeader, BuildDebugConfig(mapping))
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

		fmt.Printf("Body: %s\n", string(respBody))
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
		httpRquest.Header.Set(DebugConfigHeader, BuildDebugConfig(mapping))
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

		requestRegPassed = requestRegPassed && resp.StatusCode == request.Out.StatusCode && bytes.Equal(respBody, request.Out.Body)

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
