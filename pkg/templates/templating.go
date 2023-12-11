package templates

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/traits"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/keys"
	customparams "github.com/openshift-pipelines/pipelines-as-code/pkg/cel"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	structType = reflect.TypeOf(&structpb.Value{})
	listType   = reflect.TypeOf(&structpb.ListValue{})
	mapType    = reflect.TypeOf(&structpb.Struct{})
)

// ReplacePlaceHoldersVariables Replace those {{var}} placeholders to the runinfo variable.
func ReplacePlaceHoldersVariables(template string, dico map[string]string, rawEvent any, headers http.Header) string {
	return keys.ParamsRe.ReplaceAllStringFunc(template, func(s string) string {
		parts := keys.ParamsRe.FindStringSubmatch(s)
		key := strings.TrimSpace(parts[1])
		if strings.HasPrefix(key, "body") || strings.HasPrefix(key, "headers") {
			if rawEvent != nil && headers != nil {
				// convert headers to map[string]string
				headerMap := make(map[string]string)
				for k, v := range headers {
					headerMap[k] = v[0]
				}
				val, err := customparams.CelValue(key, rawEvent, headerMap, map[string]string{})
				if err != nil {
					return s
				}
				var raw interface{}
				var b []byte

				switch val.(type) {
				case types.String:
					if v, ok := val.Value().(string); ok {
						b = []byte(v)
					}
				case types.Bytes:
					raw, err = val.ConvertToNative(structType)
					if err == nil {
						b, err = raw.(*structpb.Value).MarshalJSON()
						if err != nil {
							b = []byte{}
						}
					}
				case types.Double, types.Int:
					raw, err = val.ConvertToNative(structType)
					if err == nil {
						b, err = raw.(*structpb.Value).MarshalJSON()
						if err != nil {
							b = []byte{}
						}
					}
				case traits.Lister:
					raw, err = val.ConvertToNative(listType)
					if err == nil {
						s, err := protojson.Marshal(raw.(proto.Message))
						if err == nil {
							b = s
						}
					}
				case traits.Mapper:
					raw, err = val.ConvertToNative(mapType)
					if err == nil {
						s, err := protojson.Marshal(raw.(proto.Message))
						if err == nil {
							b = s
						}
					}
				case types.Bool:
					raw, err = val.ConvertToNative(structType)
					if err == nil {
						b, err = json.Marshal(raw.(*structpb.Value).GetBoolValue())
						if err != nil {
							b = []byte{}
						}
					}

				default:
					raw, err = val.ConvertToNative(reflect.TypeOf([]byte{}))
					if err == nil {
						if v, ok := raw.([]byte); ok {
							b = v
						}
					}
				}
				return string(b)
			}
			return s
		}
		if _, ok := dico[key]; !ok {
			return s
		}
		return dico[key]
	})
}
