package main

import "strings"

func BuildDebugConfig(mapping map[string]string) string {
	retval := ""

	for k, v := range mapping {
		if retval != "" {
			retval += "|"
		}
		retval += k + "=" + v
	}

	return retval
}

func ParseDebugConfig(config string) map[string]string {
	shs := strings.Split(config, "|")

	retval := make(map[string]string, len(shs))

	for _, s := range shs {
		sh := strings.Split(s, "=")
		if len(sh) == 2 {
			retval[sh[0]] = sh[1]
		}
	}

	return retval
}
