package api2go

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
)

type unmarshalContext map[string]interface{}

// Unmarshal reads a JSONAPI map to a model struct
func Unmarshal(ctx unmarshalContext, values interface{}) error {
	// Check that target is a *[]Model
	ptrVal := reflect.ValueOf(values)
	if ptrVal.Kind() != reflect.Ptr || ptrVal.IsNil() {
		panic("You must pass a pointer to a []struct to Unmarshal()")
	}
	sliceType := reflect.TypeOf(values).Elem()
	sliceVal := ptrVal.Elem()
	if sliceType.Kind() != reflect.Slice {
		panic("You must pass a pointer to a []struct to Unmarshal()")
	}
	structType := sliceType.Elem()
	if structType.Kind() != reflect.Struct {
		panic("You must pass a pointer to a []struct to Unmarshal()")
	}

	// Copy the value, then write into the new variable.
	// Later Set() the actual value of the pointee.
	val := sliceVal
	err := unmarshalInto(ctx, structType, &val)
	if err != nil {
		return err
	}
	sliceVal.Set(val)
	return nil
}

func unmarshalInto(ctx unmarshalContext, structType reflect.Type, sliceVal *reflect.Value) error {
	// Read models slice
	rootName := pluralize(jsonify(structType.Name()))
	var modelsInterface interface{}
	if modelsInterface = ctx[rootName]; modelsInterface == nil {
		return errors.New("expected root document to include a '" + rootName + "' key but it didn't.")
	}
	models, ok := modelsInterface.([]interface{})
	if !ok {
		return errors.New("expected slice under key '" + rootName + "'")
	}

	// Read all the models
	for _, m := range models {
		attributes, ok := m.(map[string]interface{})
		if !ok {
			return errors.New("expected an array of objects under key '" + rootName + "'")
		}

		var val reflect.Value
		isNew := true
		id := ""

		if v := attributes["id"]; v != nil {
			id, ok = v.(string)
			if !ok {
				return errors.New("id must be a string")
			}

			// If we have an ID, check if there's already an object with that ID in the slice
			// TODO This is O(n^2), make it O(n)
			for i := 0; i < sliceVal.Len(); i++ {
				obj := sliceVal.Index(i)
				idField := obj.FieldByName("ID")
				if !idField.IsValid() {
					return errors.New("expected ID field in struct")
				}
				otherID, err := toID(idField)
				if err != nil {
					return errors.New("invalid type for ID field")
				}
				if otherID == id {
					val = obj
					isNew = false
					break
				}
			}
		}

		// Otherwise, make a new struct
		if !val.IsValid() {
			val = reflect.New(structType).Elem()

		}

		for k, v := range attributes {
			if k == "links" {
				linksMap, ok := v.(map[string]interface{})
				if !ok {
					return errors.New("expected links to be an object")
				}
				for linkName, linkVal := range linksMap {
					linkList, isASlice := linkVal.([]interface{})
					// Check for fields named 'FoobarsIDs' for key 'foobars'
					structFieldName := dejsonify(linkName) + "IDs"
					field := val.FieldByName(structFieldName)
					if !field.IsValid() {
						// no slice, check for single relation
						structFieldName = dejsonify(linkName) + "ID"
						field = val.FieldByName(structFieldName)
					}
					if !field.IsValid() {
						return errors.New("expected struct to have a " + structFieldName + " or " + structFieldName + "s field")
					}
					var kind reflect.Kind
					if field.Kind() != reflect.Slice {
						kind = field.Kind()
					} else {
						kind = field.Type().Elem().Kind()
					}
					switch kind {
					case reflect.String:
						if isASlice {
							ids := []string{}
							for _, id := range linkList {
								idString, ok := id.(string)
								if !ok {
									return errors.New("expected " + linkName + " to contain string IDs")
								}
								ids = append(ids, idString)
							}
							field.Set(reflect.ValueOf(ids))
						} else {
							idString, ok := linkVal.(string)
							if !ok {
								return errors.New("expected " + linkName + " to contain string IDs")
							}
							field.Set(reflect.ValueOf(idString))
						}

					case reflect.Int:
						if isASlice {
							ids := []int{}
							for _, id := range linkList {
								idString, ok := id.(string)
								if !ok {
									return errors.New("expected " + linkName + " to contain string IDs")
								}
								idInt, err := strconv.Atoi(idString)
								if err != nil {
									return err
								}
								ids = append(ids, idInt)
							}
							field.Set(reflect.ValueOf(ids))
						} else {
							idString, ok := linkVal.(string)
							if !ok {
								return errors.New("expected " + linkName + " to contain string IDs")
							}
							idInt, err := strconv.Atoi(idString)
							if err != nil {
								return err
							}

							field.Set(reflect.ValueOf(idInt))
						}

					default:
						return errors.New("expected " + structFieldName + " to be a int or string slice")
					}
				}
			} else if k == "id" {
				// Allow conversion of string id to int
				id, ok = v.(string)
				if !ok {
					return errors.New("expected id to be of type string")
				}
				field := val.FieldByName("ID")
				if !field.IsValid() {
					return errors.New("expected struct " + structType.Name() + " to have field 'ID'")
				}
				if field.Kind() == reflect.String {
					field.Set(reflect.ValueOf(id))
				} else if field.Kind() == reflect.Int {
					intID, err := strconv.Atoi(id)
					if err != nil {
						return err
					}
					field.Set(reflect.ValueOf(intID))
				} else {
					return errors.New("expected ID to be of type int or string in struct")
				}
			} else {
				fieldName := dejsonify(k)
				field := val.FieldByName(fieldName)
				if !field.IsValid() {
					return errors.New("expected struct " + structType.Name() + " to have field " + fieldName)
				}
				field.Set(reflect.ValueOf(v))
			}
		}

		if isNew {
			*sliceVal = reflect.Append(*sliceVal, val)
		}
	}

	return nil
}

// UnmarshalJSON reads a JSONAPI compatible JSON document to a model struct
func UnmarshalJSON(data []byte, values interface{}) error {
	var ctx unmarshalContext
	err := json.Unmarshal(data, &ctx)
	if err != nil {
		return err
	}
	return Unmarshal(ctx, values)
}
