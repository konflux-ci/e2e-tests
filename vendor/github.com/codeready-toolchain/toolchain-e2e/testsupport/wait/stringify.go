package wait

import (
	"reflect"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StringifyObject renders the object in YAML while omitting the soup of `metadata.managedFields`
func StringifyObject(obj client.Object) ([]byte, error) {
	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	unstructured.RemoveNestedField(uObj, "metadata", "managedFields")
	return yaml.Marshal(uObj)
}

// StringifyObjects renders the list of objects in YAML while omitting the soup of `metadata.managedFields`
func StringifyObjects(list client.ObjectList) ([]byte, error) {
	l := []interface{}{}
	if items := reflect.Indirect(reflect.ValueOf(list)).FieldByName("Items"); items.Kind() == reflect.Slice {
		for i := 0; i < items.Len(); i++ {
			item := items.Index(i).Interface()
			uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&item)
			if err != nil {
				return nil, err
			}
			unstructured.RemoveNestedField(uObj, "metadata", "managedFields")
			l = append(l, uObj)
		}
	}
	return yaml.Marshal(l)
}
