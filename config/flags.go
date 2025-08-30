package config

import (
	"fmt"
	"reflect"

	"github.com/spf13/pflag"
)

type PFlagExtractor[T any] struct {
	flags *pflag.FlagSet
}

func NewPFlagExtractor[T any](set *pflag.FlagSet) PFlagExtractor[T] {
	return PFlagExtractor[T]{
		flags: set,
	}
}

func (p PFlagExtractor[T]) Extract(conf *T) error {
	if !p.flags.Parsed() {
		return fmt.Errorf("pflags not parsed yet")
	}

	ref := reflect.New(reflect.TypeFor[T]())
	if ref.Kind() == reflect.Pointer {
		ref = ref.Elem()
	}

	if err := processItem(p.flags, ref, conf); err != nil {
		return err
	}

	return nil
}

func processItem[T any](flags *pflag.FlagSet, ref reflect.Value, conf *T) error {
	t := ref.Type()

	actual := reflect.ValueOf(conf).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Struct {
			if err := processItem(flags, ref.FieldByName(field.Name), conf); err != nil {
				return err
			}
		}
		tag := field.Tag.Get("flag")
		if tag == "" || tag == "-" {
			continue
		}
		value, err := getFieldValue(flags, field, tag)
		if err != nil {
			return err
		}
		actual.FieldByName(field.Name).Set(reflect.ValueOf(value))
	}

	return nil
}

func getFieldValue(flags *pflag.FlagSet, field reflect.StructField, tag string) (any, error) {
	var value any
	switch field.Type.Kind() {
	case reflect.String:
		str, err := flags.GetString(tag)
		if err != nil {
			return nil, parseError(field.Name, "string", err)
		}
		value = str

	case reflect.Int:
		in, err := flags.GetInt(tag)
		if err != nil {
			return nil, parseError(field.Name, "int", err)
		}
		value = in

	case reflect.Int8:
		in, err := flags.GetInt8(tag)
		if err != nil {
			return nil, parseError(field.Name, "int8", err)
		}
		value = in

	case reflect.Int16:
		in, err := flags.GetInt16(tag)
		if err != nil {
			return nil, parseError(field.Name, "int16", err)
		}
		value = in

	case reflect.Int32:
		in, err := flags.GetInt32(tag)
		if err != nil {
			return nil, parseError(field.Name, "int32", err)
		}
		value = in

	case reflect.Int64:
		if field.Type.String() == "time.Duration" {
			dur, err := flags.GetDuration(tag)
			if err != nil {
				return nil, parseError(field.Name, "time.Duration", err)
			}
			value = dur
			break
		}
		in, err := flags.GetInt64(tag)
		if err != nil {
			return nil, parseError(field.Name, "int64", err)
		}
		value = in

	case reflect.Float32:
		in, err := flags.GetFloat32(tag)
		if err != nil {
			return nil, parseError(field.Name, "float32", err)
		}
		value = in

	case reflect.Float64:
		in, err := flags.GetFloat64(tag)
		if err != nil {
			return nil, parseError(field.Name, "float64", err)
		}
		value = in

	default:
		return nil, fmt.Errorf("unhandled field type %s", field.Type.Kind().String())
	}

	return value, nil
}

func parseError(field string, kind string, err error) error {
	return fmt.Errorf("failed to parse %s for field %s: %w", kind, field, err)
}
