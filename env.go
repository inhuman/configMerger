package config_merger

import (
	"os"
	"reflect"
	"strconv"
	"sync"
)

type EnvSource struct {
	Variables    []string
	TargetStruct interface{}
	WatchHandler func()
}

func (e *EnvSource) Load() error {

	t := reflect.TypeOf(e.TargetStruct).Elem()
	v := reflect.ValueOf(e.TargetStruct).Elem()

	e.processEnvTags(t, v)

	return nil
}

func (e *EnvSource) processEnvTags(t reflect.Type, v reflect.Value) error {

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		if field.Type.Kind() == reflect.Struct {
			err := e.processEnvTags(field.Type, value)
			if err != nil {
				return err
			}
			continue
		}

		column := field.Tag.Get("env")



		if (column != "") && (StringInSlice(column, e.Variables)) {
			os.Getenv(column)

			//TODO: add float type, just in case
			v := os.Getenv(column)

			switch value.Kind() {
			case reflect.String:
				value.SetString(v)

			case reflect.Int:
				i, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				value.SetInt(i)

			case reflect.Bool:
				b, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				value.SetBool(b)
			}
		}
	}
	return nil
}


func (e *EnvSource) Watch(done chan bool, group *sync.WaitGroup) {
	<-done
}

func (e *EnvSource) SetTargetStruct(i interface{}) {
	e.TargetStruct = i
}
