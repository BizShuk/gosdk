package utils

import "reflect"

func IsNil(p any) bool {
	return p == nil ||
		(reflect.ValueOf(p).Kind() == reflect.Ptr && reflect.ValueOf(p).IsNil())
}
