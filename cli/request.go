package main

import (
	"fmt"
	"strings"
)

func BuildRequest(rc string, records []Record) Request {
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

func PrintRequest(request Request, statusCode, level int) {
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
		PrintRequest(request.Dependencies[i].Reference, request.Dependencies[i].Out.StatusCode, level+1)
	}
}

func buildRequestTree(records []Record, ec string) Request {
	req := Request{
		Dependencies: make([]Dependency, len(records)),
		Observations: make([]Record, len(records)),
	}

	notUsed := make([]Record, 0, len(records))

	maxGsq := -1
	maxOsq := -1

	for i := range records {
		if records[i].ExecutionContext == ec {
			switch records[i].RecordType {
			case RequestRecordType:
				req.In = records[i]
			case ResponseRecordType:
				req.Out = records[i]
			case DependencyRequestRecordType:
				req.Dependencies[records[i].DepencencySequence].In = records[i]
				if records[i].DepencencySequence > maxGsq {
					maxGsq = records[i].DepencencySequence
				}
			case DependencyResponseRecordType:
				req.Dependencies[records[i].DepencencySequence].Out = records[i]
				if records[i].DepencencySequence > maxGsq {
					maxGsq = records[i].DepencencySequence
				}
			case ObservedRecordType:
				req.Observations[records[i].ObservationSequence] = records[i]
				if records[i].ObservationSequence > maxOsq {
					maxOsq = records[i].ObservationSequence
				}
			default:
				fmt.Printf("Unknown record %v\n", records[i])
			}
		} else {
			notUsed = append(notUsed, records[i])
		}
	}

	req.Dependencies = req.Dependencies[0 : maxGsq+1]
	req.Observations = req.Observations[0 : maxOsq+1]

	for i := range req.Dependencies {
		if req.Dependencies[i].In.DependencyContext != "" {
			req.Dependencies[i].Reference = buildRequestTree(notUsed, req.Dependencies[i].In.DependencyContext)
		}
	}

	return req
}

func getPreposition(level int) string {
	return strings.Join(make([]string, level+1), "    ")
}
