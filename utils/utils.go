package utils

import "reflect"

func Contains(slice, elem interface{}) bool {

	sv := reflect.ValueOf(slice)

	// Check that slice is actually a slice/array.
	// you might want to return an error here
	if sv.Kind() != reflect.Slice && sv.Kind() != reflect.Array {
		return false
	}

	// iterate the slice
	for i := 0; i < sv.Len(); i++ {

		// compare elem to the current slice element
		if elem == sv.Index(i).Interface() {
			return true
		}
	}

	// nothing found
	return false

}
