package main

import "fmt"

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
