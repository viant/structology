package conv

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"time"
)

type SimpleStruct struct {
	Name        string
	Age         int
	Active      bool
	Score       float64
	DateJoined  time.Time
	Tags        []string
	Collection  []*Basic
	IgnoreField string `json:"-"`
	Renamed     string `json:"custom_name"`
	unexported  string
}

type Basic struct {
	Id int
}

type nestedStruct struct {
	SimpleStruct
	Address string
	Details map[string]interface{}
}

func TestConvertToString(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	testCases := []struct {
		name     string
		src      interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"int", 123, "123"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"float", 123.456, "123.456"},
		{"bytes", []byte("hello"), "hello"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := converter.Convert(tc.src, &result)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestConvertToBool(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	testCases := []struct {
		name     string
		src      interface{}
		expected bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"int 1", 1, true},
		{"int 0", 0, false},
		{"string true", "true", true},
		{"string false", "false", false},
		{"string 1", "1", true},
		{"string 0", "0", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result bool
			err := converter.Convert(tc.src, &result)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestConvertToInt(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	testCases := []struct {
		name     string
		src      interface{}
		expected int
	}{
		{"int", 123, 123},
		{"int8", int8(8), 8},
		{"int16", int16(16), 16},
		{"int32", int32(32), 32},
		{"int64", int64(64), 64},
		{"uint", uint(123), 123},
		{"float32", float32(123.5), 123},
		{"float64", 123.5, 123},
		{"string", "123", 123},
		{"string float", "123.5", 123},
		{"bool true", true, 1},
		{"bool false", false, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result int
			err := converter.Convert(tc.src, &result)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestConvertToFloat(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	testCases := []struct {
		name     string
		src      interface{}
		expected float64
	}{
		{"int", 123, 123.0},
		{"float32", float32(123.5), 123.5},
		{"float64", 123.5, 123.5},
		{"string", "123.5", 123.5},
		{"bool true", true, 1.0},
		{"bool false", false, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result float64
			err := converter.Convert(tc.src, &result)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestConvertToTime(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	// Reference time
	refTime := time.Date(2023, 1, 15, 12, 30, 45, 0, time.UTC)

	testCases := []struct {
		name     string
		src      interface{}
		expected time.Time
	}{
		{"RFC3339", "2023-01-15T12:30:45Z", refTime},
		{"custom format", "2023-01-15 12:30:45.000", refTime},
		{"unix timestamp", int64(refTime.Unix()), refTime},
		{"time.Time", refTime, refTime},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result time.Time
			err := converter.Convert(tc.src, &result)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			if !result.Equal(tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestConvertToSlice(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	testCases := []struct {
		name     string
		src      interface{}
		dest     interface{}
		expected interface{}
	}{
		{
			"[]int from []int",
			[]int{1, 2, 3},
			&[]int{},
			[]int{1, 2, 3},
		},
		{
			"[]int from []float64",
			[]float64{1.1, 2.2, 3.3},
			&[]int{},
			[]int{1, 2, 3},
		},
		{
			"[]string from []int",
			[]int{1, 2, 3},
			&[]string{},
			[]string{"1", "2", "3"},
		},
		{
			"[]string from string",
			"hello",
			&[]string{},
			[]string{"hello"},
		},
		{
			"[]string from []interface{}",
			[]interface{}{"hello", 123, true, 45.67},
			&[]string{},
			[]string{"hello", "123", "true", "45.67"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := converter.Convert(tc.src, tc.dest)
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}

			destValue := reflect.ValueOf(tc.dest).Elem().Interface()
			if !reflect.DeepEqual(destValue, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, destValue)
			}
		})
	}
}

func TestConvertToMap(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	// Test struct to map
	type Person struct {
		Name  string  `json:"name"`
		Age   int     `json:"age"`
		Score float64 `json:"score"`
	}

	person := Person{
		Name:  "John",
		Age:   30,
		Score: 85.5,
	}

	expectedMap := map[string]interface{}{
		"Name":  "John",
		"Age":   30,
		"Score": 85.5,
	}

	var resultMap map[string]interface{}
	err := converter.Convert(person, &resultMap)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	// Check key by key since map comparisons can be tricky
	if len(resultMap) != len(expectedMap) {
		t.Errorf("Map length mismatch, expected %d, got %d", len(expectedMap), len(resultMap))
	}

	for k, v := range expectedMap {
		if resultMap[k] != v {
			t.Errorf("Key %s: expected %v, got %v", k, v, resultMap[k])
		}
	}

	// Test map to map
	srcMap := map[string]int{
		"one": 1,
		"two": 2,
	}

	var destMap map[string]string
	err = converter.Convert(srcMap, &destMap)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	expectedStringMap := map[string]string{
		"one": "1",
		"two": "2",
	}

	if !reflect.DeepEqual(destMap, expectedStringMap) {
		t.Errorf("Expected %v, got %v", expectedStringMap, destMap)
	}
}

func TestConvertMapToStruct(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	// Source map
	srcMap := map[string]interface{}{
		"name":        "Jane Smith",
		"age":         25,
		"active":      true,
		"score":       92.5,
		"custom_name": "Renamed Value",
		"tags":        []string{"tag1", "tag2"},
		"collection": []interface{}{
			map[string]interface{}{
				"id": 1,
			},
			map[string]interface{}{
				"id": 2,
			},
		},
	}

	var result SimpleStruct
	err := converter.Convert(srcMap, &result)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	// Validate converted values
	if result.Name != "Jane Smith" {
		t.Errorf("Name: expected 'Jane Smith', got %q", result.Name)
	}

	if result.Age != 25 {
		t.Errorf("Age: expected 25, got %d", result.Age)
	}

	if !result.Active {
		t.Errorf("Active: expected true, got false")
	}

	if result.Score != 92.5 {
		t.Errorf("Score: expected 92.5, got %f", result.Score)
	}

	if !assert.EqualValues(t, result.Tags, []string{"tag1", "tag2"}) {
		t.Errorf("Tags: expected tag1,tag2, got %v", result.Tags)
	}

	if !assert.EqualValues(t, result.Collection, []*Basic{{Id: 1}, {Id: 2}}) {
		t.Errorf("Tags: expected tag1,tag2, got %v", result.Tags)
	}
	if result.Renamed != "Renamed Value" {
		t.Errorf("Renamed: expected 'Renamed Value', got %q", result.Renamed)
	}

	// Test case sensitivity
	caseSensitiveOpts := DefaultOptions()
	caseSensitiveOpts.CaseSensitive = true
	caseConverter := NewConverter(caseSensitiveOpts)

	mixedCaseMap := map[string]interface{}{
		"Name": "John Doe",
		"age":  30,
	}

	var caseResult SimpleStruct
	err = caseConverter.Convert(mixedCaseMap, &caseResult)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	if caseResult.Name != "John Doe" {
		t.Errorf("Case sensitive - Name: expected 'John Doe', got %q", caseResult.Name)
	}

	// Age would be zero with case sensitivity on
	if caseResult.Age != 0 {
		t.Errorf("Case sensitive - Age: expected 0, got %d", caseResult.Age)
	}
}

func TestConvertNestedStructs(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	// Source map with nested data
	srcMap := map[string]interface{}{
		"name":    "John Doe",
		"age":     30,
		"active":  true,
		"address": "123 Main St",
		"details": map[string]interface{}{
			"country": "USA",
			"zip":     "12345",
		},
	}

	var result nestedStruct
	err := converter.Convert(srcMap, &result)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	// Validate converted values
	if result.Name != "John Doe" {
		t.Errorf("Name: expected 'John Doe', got %q", result.Name)
	}

	if result.Age != 30 {
		t.Errorf("Age: expected 30, got %d", result.Age)
	}

	if !result.Active {
		t.Errorf("Active: expected true, got false")
	}

	if result.Address != "123 Main St" {
		t.Errorf("Address: expected '123 Main St', got %q", result.Address)
	}

	if result.Details["country"] != "USA" {
		t.Errorf("Details.country: expected 'USA', got %v", result.Details["country"])
	}

	if result.Details["zip"] != "12345" {
		t.Errorf("Details.zip: expected '12345', got %v", result.Details["zip"])
	}
}

func TestCustomConversion(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	// Register custom conversion
	converter.RegisterConversion(
		reflect.TypeOf(""),
		reflect.TypeOf([]int{}),
		ConversionFunc(func(src interface{}, dest interface{}, opts Options) error {
			str := src.(string)
			result := make([]int, len(str))
			for i, r := range str {
				result[i] = int(r)
			}

			// Set the result to the destination
			destVal := reflect.ValueOf(dest).Elem()
			destVal.Set(reflect.ValueOf(result))

			return nil
		}),
	)

	var result []int
	err := converter.Convert("abc", &result)
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}

	expected := []int{97, 98, 99} // ASCII for 'abc'
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestProjectsConversion(t *testing.T) {
	converter := NewConverter(DefaultOptions())

	// Test case with Projects as []interface{} of map[string]interface{}
	type Project struct {
		RootPath     string
		Type         string
		Name         string
		RelativePath string
	}

	type Config struct {
		Projects []*Project
	}

	// Sample data from the requirement
	srcData := map[string]interface{}{
		"Projects": []interface{}{
			map[string]interface{}{
				"RootPath":     "fluxor",
				"Type":         "go",
				"Name":         "github.com/viant/fluxor",
				"RelativePath": ".",
			},
		},
	}

	var config Config
	err := converter.Convert(srcData, &config)
	if err != nil {
		t.Fatalf("Failed to convert to Config struct: %v", err)
	}

	// Verify the conversion was successful
	if len(config.Projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(config.Projects))
	}

	project := config.Projects[0]

	assert.Equal(t, "fluxor", project.RootPath)
	assert.Equal(t, "go", project.Type)
	assert.Equal(t, "github.com/viant/fluxor", project.Name)
	assert.Equal(t, ".", project.RelativePath)
}
