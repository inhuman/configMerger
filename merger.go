package config_merger

import (
	"fmt"
	"github.com/fatih/structs"
	"github.com/hashicorp/go-multierror"
	"log"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"
)

type Merger struct {
	Sources            []Source
	TargetConfigStruct interface{}
	done               chan bool
}

type SourceModel struct {
	TagIds       map[string]string
	TargetStruct interface{}
	WatchHandler func()
}

type Source interface {
	Load() error
	SetTargetStruct(s interface{})
	GetTagIds() map[string]string
	Watch(done chan bool, group *sync.WaitGroup)
}

func NewMerger(s interface{}) *Merger {
	m := &Merger{
		done: make(chan bool),
	}

	if reflect.ValueOf(s).Kind() != reflect.Ptr {
		panic(fmt.Sprintf("must provide pointer to struct, received [%T]", s))
	}

	err := validateStruct(s)
	if err != nil {
		panic(err.Error())
	}

	m.TargetConfigStruct = s
	return m
}

func (m *Merger) AddSource(src Source) {
	src.SetTargetStruct(m.TargetConfigStruct)
	m.Sources = append(m.Sources, src)
}

func (m *Merger) RunWatch() error {

	var errAll *multierror.Error

	var wg sync.WaitGroup

	doneMap := make(map[int]chan bool)

	for i, s := range m.Sources {
		err := s.Load()
		if err != nil {
			errAll = multierror.Append(errAll, err)
		}
		doneMap[i] = make(chan bool)
		go s.Watch(doneMap[i], &wg)
	}

	if errAll != nil {
		if len(errAll.Errors) > 0 {
			return errAll
		}
	}
	<-m.done

	for d := range m.Sources {
		doneMap[d] <- true
	}
	wg.Wait()
	return nil
}

func (m *Merger) StopWatch() {
	m.done <- true
}

func (m *Merger) Run() error {

	var errAll *multierror.Error

	for _, s := range m.Sources {

		log.Println("loading source:", reflect.TypeOf(s).String())

		err := s.Load()
		if err != nil {
			log.Println("loading source:", reflect.TypeOf(s).String())

			errAll = multierror.Append(errAll, err)
		}
	}

	if errAll != nil {
		if len(errAll.Errors) > 0 {
			return errAll
		}
	}
	err := m.setDefaults()
	if err != nil {
		return err
	}
	err = m.checkRequiredFields()

	if err != nil {
		return err
	}

	return nil
}

func (m *Merger) GetFinalConfig() map[string]interface{} {
	return structs.Map(m.TargetConfigStruct)
}

func (m *Merger) PrintConfig() {

	t := reflect.TypeOf(m.TargetConfigStruct).Elem()
	v := reflect.ValueOf(m.TargetConfigStruct).Elem()

	fmt.Println(reflect.TypeOf(m.TargetConfigStruct))

	processPrint(t, v, "  ")

}

func processPrint(t reflect.Type, v reflect.Value, offset string) {

	if t.Kind() == reflect.Map {

		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()
			value := iter.Value()

			switch reflect.ValueOf(value.Interface()).Kind() {

			case reflect.Map:
				fmt.Println(offset + k.String() + ":")
				processPrint(reflect.ValueOf(value.Interface()).Type(), reflect.ValueOf(value.Interface()), offset+"  ")
			default:
				fmt.Printf(offset+k.String()+": %+v\n", value)
			}
		}

	} else {

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			value := v.Field(i)

			if field.Type.Kind() == reflect.Struct {
				fmt.Println(offset + field.Name + ":")
				processPrint(field.Type, value, offset+"  ")

			} else if field.Type.Kind() == reflect.Ptr {
				field.Type = field.Type.Elem()
				value = value.Elem()
				fmt.Println(offset + field.Name + ":")

				processPrint(field.Type, value, offset+"  ")

			} else {

				column := field.Tag.Get("show_last_symbols")
				if column != "" {

					maskLast, err := strconv.Atoi(column)
					if err == nil {
						fmt.Println(offset + field.Name + ": " + maskString(value.String(), maskLast))
					} else {
						fmt.Println(err)
					}

				} else {

					switch value.Kind() {
					case reflect.String:
						fmt.Println(offset + field.Name + ": " + value.String())
					case reflect.Int:
						fmt.Printf(offset+field.Name+": %d\n", value.Int())
					case reflect.Bool:
						fmt.Printf(offset+field.Name+": %t\n", value.Bool())
					case reflect.Map:
						fmt.Println(offset + field.Name + ":")
						processPrint(field.Type, value, offset+"  ")
					}
				}
			}
		}
	}

}

func (m *Merger) StopDisconnectTimeout(address string, timeout time.Duration) {

	go func() {
		for {
			conn, err := net.Dial("tcp", address)
			if err != nil {
				fmt.Errorf("TCP error : %s", err.Error())
			}
			if conn == nil {
				fmt.Println("can not reach server")
				m.StopWatch()
			}
			<-time.After(timeout * time.Second)
		}
	}()
}
