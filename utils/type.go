package utils

import "reflect"

func IsNil(p interface{}) bool {
	return p == nil ||
		(reflect.ValueOf(p).Kind() == reflect.Ptr && reflect.ValueOf(p).IsNil())
}
