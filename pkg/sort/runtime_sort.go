package sort

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/fvbommel/sortorder"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/klog/v2"
	"k8s.io/utils/integer"
)

// The following code has been take from kubectl get command
// instead of importing all the dependencies, copying only the required part
// https://github.com/kubernetes/kubernetes/blob/20d0ab7ae808aaddb1556c3c38ca0607663c50ac/staging/src/k8s.io/kubectl/pkg/cmd/get/sorter.go#L150

// RuntimeSort is an implementation of the golang sort interface that knows how to sort
// lists of runtime.Object.
type RuntimeSort struct {
	field        string
	objs         []runtime.Object
	origPosition []int
}

// NewRuntimeSort creates a new RuntimeSort struct that implements golang sort interface.
func NewRuntimeSort(field string, objs []runtime.Object) *RuntimeSort {
	sorter := &RuntimeSort{field: field, objs: objs, origPosition: make([]int, len(objs))}
	for ix := range objs {
		sorter.origPosition[ix] = ix
	}
	return sorter
}

func (r *RuntimeSort) Len() int {
	return len(r.objs)
}

func (r *RuntimeSort) Swap(i, j int) {
	r.objs[i], r.objs[j] = r.objs[j], r.objs[i]
	r.origPosition[i], r.origPosition[j] = r.origPosition[j], r.origPosition[i]
}

func isLess(i, j reflect.Value) (bool, error) {
	//nolint
	switch i.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return i.Int() < j.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return i.Uint() < j.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return i.Float() < j.Float(), nil
	case reflect.String:
		return sortorder.NaturalLess(i.String(), j.String()), nil
	case reflect.Pointer:
		return isLess(i.Elem(), j.Elem())
	case reflect.Struct:
		// sort metav1.Time
		in := i.Interface()
		if t, ok := in.(metav1.Time); ok {
			time := j.Interface().(metav1.Time)
			return t.Before(&time), nil
		}
		// sort resource.Quantity
		if iQuantity, ok := in.(resource.Quantity); ok {
			jQuantity := j.Interface().(resource.Quantity)
			return iQuantity.Cmp(jQuantity) < 0, nil
		}
		// fallback to the fields comparison
		for idx := 0; idx < i.NumField(); idx++ {
			less, err := isLess(i.Field(idx), j.Field(idx))
			if err != nil || !less {
				return less, err
			}
		}
		return true, nil
	case reflect.Array, reflect.Slice:
		// note: the length of i and j may be different
		for idx := 0; idx < integer.IntMin(i.Len(), j.Len()); idx++ {
			less, err := isLess(i.Index(idx), j.Index(idx))
			if err != nil || !less {
				return less, err
			}
		}
		return true, nil
	case reflect.Interface:
		//nolint
		if i.IsNil() && j.IsNil() {
			return false, nil
		} else if i.IsNil() {
			return true, nil
		} else if j.IsNil() {
			return false, nil
		}
		switch itype := i.Interface().(type) {
		case uint8:
			if jtype, ok := j.Interface().(uint8); ok {
				return itype < jtype, nil
			}
		case uint16:
			if jtype, ok := j.Interface().(uint16); ok {
				return itype < jtype, nil
			}
		case uint32:
			if jtype, ok := j.Interface().(uint32); ok {
				return itype < jtype, nil
			}
		case uint64:
			if jtype, ok := j.Interface().(uint64); ok {
				return itype < jtype, nil
			}
		case int8:
			if jtype, ok := j.Interface().(int8); ok {
				return itype < jtype, nil
			}
		case int16:
			if jtype, ok := j.Interface().(int16); ok {
				return itype < jtype, nil
			}
		case int32:
			if jtype, ok := j.Interface().(int32); ok {
				return itype < jtype, nil
			}
		case int64:
			if jtype, ok := j.Interface().(int64); ok {
				return itype < jtype, nil
			}
		case uint:
			if jtype, ok := j.Interface().(uint); ok {
				return itype < jtype, nil
			}
		case int:
			if jtype, ok := j.Interface().(int); ok {
				return itype < jtype, nil
			}
		case float32:
			if jtype, ok := j.Interface().(float32); ok {
				return itype < jtype, nil
			}
		case float64:
			if jtype, ok := j.Interface().(float64); ok {
				return itype < jtype, nil
			}
		case string:
			if jtype, ok := j.Interface().(string); ok {
				// check if it's a Quantity
				itypeQuantity, err := resource.ParseQuantity(itype)
				if err != nil {
					return sortorder.NaturalLess(itype, jtype), nil
				}
				jtypeQuantity, err := resource.ParseQuantity(jtype)
				if err != nil {
					return sortorder.NaturalLess(itype, jtype), nil
				}
				// Both strings are quantity
				return itypeQuantity.Cmp(jtypeQuantity) < 0, nil
			}
		default:
			return false, fmt.Errorf("unsortable type: %T", itype)
		}
		return false, fmt.Errorf("unsortable interface: %v", i.Kind())

	default:
		return false, fmt.Errorf("unsortable type: %v", i.Kind())
	}
}

func (r *RuntimeSort) Less(i, j int) bool {
	iObj := r.objs[i]
	jObj := r.objs[j]

	var iValues [][]reflect.Value
	var jValues [][]reflect.Value
	var err error

	parser := jsonpath.New("sorting").AllowMissingKeys(true)
	err = parser.Parse(r.field)
	if err != nil {
		panic(err)
	}

	iValues, err = findJSONPathResults(parser, iObj)
	if err != nil {
		klog.Fatalf("Failed to get i values for %#v using %s (%#v)", iObj, r.field, err)
	}

	jValues, err = findJSONPathResults(parser, jObj)
	if err != nil {
		klog.Fatalf("Failed to get j values for %#v using %s (%v)", jObj, r.field, err)
	}

	if len(iValues) == 0 || len(iValues[0]) == 0 {
		return true
	}
	if len(jValues) == 0 || len(jValues[0]) == 0 {
		return false
	}
	iField := iValues[0][0]
	jField := jValues[0][0]

	less, err := isLess(iField, jField)
	if err != nil {
		klog.Exitf("Field %s in %T is an unsortable type: %s, err: %v", r.field, iObj, iField.Kind().String(), err)
	}
	return less
}

// OriginalPosition returns the starting (original) position of a particular index.
// e.g. If OriginalPosition(0) returns 5 than the
// item currently at position 0 was at position 5 in the original unsorted array.
func (r *RuntimeSort) OriginalPosition(ix int) int {
	if ix < 0 || ix > len(r.origPosition) {
		return -1
	}
	return r.origPosition[ix]
}

func findJSONPathResults(parser *jsonpath.JSONPath, from runtime.Object) ([][]reflect.Value, error) {
	if unstructuredObj, ok := from.(*unstructured.Unstructured); ok {
		return parser.FindResults(unstructuredObj.Object)
	}
	return parser.FindResults(reflect.ValueOf(from).Elem().Interface())
}

// ByField sorts the runtime objects passed by the field.
func ByField(field string, runTimeObj []runtime.Object) {
	sorter := NewRuntimeSort(field, runTimeObj)
	if len(runTimeObj) > 0 {
		sort.Sort(sorter)
	}
}
