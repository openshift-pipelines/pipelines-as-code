package bitbucketdatacenter

import (
	"fmt"

	bbv1 "github.com/gfleury/go-bitbucket-v1"
)

type apiResultfunc func(int) (*bbv1.APIResponse, error)

// paginate go over an API call and fetch next results.
func paginate(apiResultfunc apiResultfunc) ([]any, error) {
	var nextPageStart int

	allValues := []any{}
	for {
		result, err := apiResultfunc(nextPageStart)
		if err != nil {
			return nil, err
		}

		if result.Payload != nil {
			// I know I know your eyebrow is 🤨, the lib sometime return the payload and sometime parsed values.
			// so we just return the raw payload and we will handle it in the caller
			allValues = append(allValues, result.Payload)
		} else {
			if result.Values["values"] == nil {
				return nil, fmt.Errorf("key \"values\" not found in result")
			}
			values, ok := result.Values["values"].([]any)
			if !ok {
				return nil, fmt.Errorf("key \"values\" is not an array")
			}
			allValues = append(allValues, values...)
		}
		np, ok := result.Values["nextPageStart"].(float64)
		if !ok {
			break
		}
		nextPageStart = int(np)

		isLastPage, ok := result.Values["isLastPage"]
		if !ok {
			break
		}
		isLastPageb, ok := isLastPage.(bool)
		if !ok {
			break
		}

		if isLastPageb {
			break
		}
	}
	return allValues, nil
}
