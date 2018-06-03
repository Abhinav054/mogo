package bongo

import (
	"fmt"

	"github.com/go-bongo/go-dotaccess"
	// "github.com/go-bongo/mgo/bson"
	"reflect"
	"strings"
)

// DiffTracker ...
type DiffTracker struct {
	original interface{}
	current  interface{}
}

// Trackable ...
type Trackable interface {
	GetDiffTracker() *DiffTracker
}

// NewDiffTracker ...
func NewDiffTracker(doc interface{}) *DiffTracker {
	c := &DiffTracker{
		current:  doc,
		original: nil,
	}

	return c
}

// DiffTrackingSession ...
type DiffTrackingSession struct {
	ChangedFields []string
	IsNew         bool
}

// NewSession ...
func (d *DiffTracker) NewSession(useBsonTags bool) (*DiffTrackingSession, error) {
	sess := &DiffTrackingSession{}

	isNew, changedFields, err := d.Compare(useBsonTags)

	sess.IsNew = isNew
	sess.ChangedFields = changedFields

	return sess, err
}

// Reset ...
func (d *DiffTracker) Reset() {
	// Store a copy of current
	d.original = reflect.Indirect(reflect.ValueOf(d.current)).Interface()
}

// Modified ...
func (s *DiffTrackingSession) Modified(field string) bool {

	if s.IsNew {
		return true
	}

	for _, d := range s.ChangedFields {
		if d == field || strings.HasPrefix(d, field+".") {
			return true
		}
	}
	return false
}

// Modified test on DiffTracker struct...
func (d *DiffTracker) Modified(field string) bool {
	sess, _ := d.NewSession(false)
	return sess.Modified(field)
}

// GetModified ...
func (d *DiffTracker) GetModified(useBson bool) (bool, []string) {
	isNew, diffs, _ := d.Compare(useBson)

	return isNew, diffs
}

// GetOriginalValue ...
func (d *DiffTracker) GetOriginalValue(field string) (interface{}, error) {
	if d.original != nil {
		return dotaccess.Get(d.original, field)
	}
	return nil, nil

}

// SetOriginal ...
func (d *DiffTracker) SetOriginal(orig interface{}) {
	d.original = reflect.Indirect(reflect.ValueOf(orig)).Interface()
}

// Clear ...
func (d *DiffTracker) Clear() {
	d.original = nil
}

// Compare ...
func (d *DiffTracker) Compare(useBson bool) (bool, []string, error) {
	defer func() {

		if r := recover(); r != nil {
			fmt.Println("You probably forgot to initialize the DiffTracker instance on your model")
			panic(r)
		}
	}()
	if d.original != nil {
		diffs, err := GetChangedFields(d.original, d.current, useBson)
		return false, diffs, err
	}
	return true, []string{}, nil
}

func getFields(t reflect.Type) []string {
	fields := []string{}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fields = append(fields, field.Name)
	}

	return fields

}

func isNilOrInvalid(f reflect.Value) bool {
	if f.Kind() == reflect.Ptr && f.IsNil() {
		return true
	}
	return (!f.IsValid())
}

// Stringer ...
type Stringer interface {
	String() string
}

// GetChangedFields ...
func GetChangedFields(struct1 interface{}, struct2 interface{}, useBson bool) ([]string, error) {

	diffs := make([]string, 0)
	val1 := reflect.ValueOf(struct1)
	type1 := val1.Type()

	val2 := reflect.ValueOf(struct2)
	type2 := val2.Type()

	if type1.Kind() == reflect.Ptr {
		type1 = type1.Elem()
		val1 = val1.Elem()
	}
	if type2.Kind() == reflect.Ptr {
		type2 = type2.Elem()
		val2 = val2.Elem()
	}

	if type1.String() != type2.String() {
		return diffs, fmt.Errorf("Cannot compare two structs of different types %s and %s", type1.String(), type2.String())
	}

	if type1.Kind() != reflect.Struct || type2.Kind() != reflect.Struct {
		return diffs, fmt.Errorf("Can only compare two structs or two pointers to structs (got %s and %s)", type1.Kind(), type2.Kind())
	}

	for i := 0; i < type1.NumField(); i++ {

		field1 := val1.Field(i)
		field2 := val2.Field(i)

		field := type1.Field(i)
		tags := strings.Split(field.Tag.Get("bson"), ",")
		inline := false
		for _, t := range tags {
			if t == "inline" {
				inline = true
				break
			}
		}

		var fieldName string
		if useBson {
			fieldName = GetBsonName(field)
		} else {
			fieldName = field.Name
		}

		childType := field1.Type()
		// Recurse?
		if childType.Kind() == reflect.Ptr {
			childType = childType.Elem()
		}

		// Skip if not exported
		if len(field.PkgPath) > 0 {
			continue
		}

		if childType.Kind() == reflect.Struct {

			var childDiffs []string
			var err error
			// Make sure they aren't zero-value. Skip if so
			if isNilOrInvalid(field1) && isNilOrInvalid(field2) {
				continue
			} else if isNilOrInvalid(field1) || isNilOrInvalid(field2) {
				childDiffs = getFields(childType)

			} else {
				if _, ok := field1.Interface().(Stringer); ok {
					if fmt.Sprint(field1.Interface()) != fmt.Sprint(field2.Interface()) {
						diffs = append(diffs, fieldName)
					}

				} else {
					childDiffs, err = GetChangedFields(field1.Interface(), field2.Interface(), useBson)

					if err != nil {
						return diffs, err
					}
				}

			}

			if len(childDiffs) > 0 {
				for _, diff := range childDiffs {
					if inline {
						diffs = append(diffs, diff)
					} else {
						diffs = append(diffs, strings.Join([]string{fieldName, diff}, "."))
					}

				}
			}
		} else {

			if fmt.Sprint(field1.Interface()) != fmt.Sprint(field2.Interface()) {
				diffs = append(diffs, fieldName)
			}
		}
	}

	return diffs, nil

}
