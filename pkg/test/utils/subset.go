package utils

import (
	"fmt"
	"reflect"
)

type ArrayComparisonStrategyFactory = func(path string) ArrayComparisonStrategy
type ArrayComparisonStrategy = func(expectedData, actualData interface{}) error

func DefaultStrategyFactory() ArrayComparisonStrategyFactory {
	return alwaysExact
}

func alwaysExact(path string) ArrayComparisonStrategy {
	return StrategyExact(path, DefaultStrategyFactory())
}

// SubsetError is an error type used by IsSubset for tracking the path in the struct.
type SubsetError struct {
	path    []string
	message string
}

// AppendPath appends key to the existing struct path. For example, in struct member `a.Key1.Key2`, the path would be ["Key1", "Key2"]
func (e *SubsetError) AppendPath(key string) {
	if e.path == nil {
		e.path = []string{}
	}

	e.path = append(e.path, key)
}

// Error implements the error interface.
func (e *SubsetError) Error() string {
	if e.path == nil || len(e.path) == 0 {
		return e.message
	}

	path := ""
	for i := len(e.path) - 1; i >= 0; i-- {
		path = fmt.Sprintf("%s.%s", path, e.path[i])
	}

	return fmt.Sprintf("%s: %s", path, e.message)
}

// IsSubset checks to see if `expected` is a subset of `actual`. A "subset" is an object that is equivalent to
// the other object, but where map keys found in actual that are not defined in expected are ignored.
func IsSubset(expected, actual interface{}, currentPath string, strategyFactory ArrayComparisonStrategyFactory) error {
	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		return &SubsetError{
			message: fmt.Sprintf("type mismatch: %v != %v", reflect.TypeOf(expected), reflect.TypeOf(actual)),
		}
	}

	if reflect.DeepEqual(expected, actual) {
		return nil
	}

	switch reflect.TypeOf(expected).Kind() {
	case reflect.Slice:
		if strategyFactory == nil {
			strategyFactory = DefaultStrategyFactory()
		}
		strategy := strategyFactory(currentPath)

		return strategy(expected, actual)

	case reflect.Map:
		iter := reflect.ValueOf(expected).MapRange()

		for iter.Next() {
			actualValue := reflect.ValueOf(actual).MapIndex(iter.Key())

			if !actualValue.IsValid() {
				return &SubsetError{
					path:    []string{iter.Key().String()},
					message: "key is missing from map",
				}
			}

			newPath := currentPath + "/" + iter.Key().String()
			if err := IsSubset(iter.Value().Interface(), actualValue.Interface(), newPath, strategyFactory); err != nil {
				subsetErr, ok := err.(*SubsetError)
				if ok {
					subsetErr.AppendPath(iter.Key().String())
					return subsetErr
				}
				return err
			}
		}
	default:
		return &SubsetError{
			message: fmt.Sprintf("value mismatch, expected: %v != actual: %v", expected, actual),
		}
	}

	return nil
}

func StrategyAnywhere(path string, strategyFactory ArrayComparisonStrategyFactory) ArrayComparisonStrategy {
	return func(expected, actual interface{}) error {
		expectedData := toSlice(expected)
		actualData := toSlice(actual)

		for i, expectedItem := range expectedData {
			matched := false
			for _, actualItem := range actualData {
				newPath := path + fmt.Sprintf("[%d]", i)
				if err := IsSubset(expectedItem, actualItem, newPath, strategyFactory); err == nil {
					matched = true
					break
				}
			}
			if !matched {
				return &SubsetError{message: fmt.Sprintf("expected item %v not found in actual slice at path %s", expectedItem, path)}
			}
		}
		return nil
	}
}

func StrategyExact(path string, strategyFactory ArrayComparisonStrategyFactory) ArrayComparisonStrategy {
	return func(expected, actual interface{}) error {
		if reflect.ValueOf(expected).Len() != reflect.ValueOf(actual).Len() {
			return &SubsetError{message: fmt.Sprintf("slice length mismatch at path %s: %d != %d", path, reflect.ValueOf(expected).Len(), reflect.ValueOf(actual).Len())}
		}
		for i := 0; i < reflect.ValueOf(expected).Len(); i++ {
			newPath := path + fmt.Sprintf("[%d]", i)
			if err := IsSubset(reflect.ValueOf(expected).Index(i).Interface(), reflect.ValueOf(actual).Index(i).Interface(), newPath, strategyFactory); err != nil {
				return err
			}
		}
		return nil
	}
}

func toSlice(v interface{}) []interface{} {
	value := reflect.ValueOf(v)
	slice := make([]interface{}, value.Len())
	for i := 0; i < value.Len(); i++ {
		slice[i] = value.Index(i).Interface()
	}
	return slice
}
