package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"reflect"
	"regexp"

	"github.com/codegangsta/inject"
	"github.com/golang/glog"
)

const Defaultconfig = `
{
    "input": [
        {
            "type": "stdin"
        }
    ],
    "filter": [
        {
            "type": "zeus",
            "key": "foo",
            "value": "bar"
        }
    ],
    "output": [
        {
            "type": "stdout"
        }
    ]
}
`

// Config struct for the logcool.
type TypeConfig interface {
	SetInjector(inj inject.Injector)
	GetType() string
	Invoke(f interface{}) (refvs []reflect.Value, err error)
}

// Common config for logcool.
type CommonConfig struct {
	inject.Injector `json:"-"`
	Type            string `json:"type"`
}

// config raw type.
type ConfigRaw map[string]interface{}

// config struct for config-raw.
type Config struct {
	inject.Injector `json:"-"`
	InputRaw        []ConfigRaw `json:"input"`
	FilterRaw       []ConfigRaw `json:"filter"`
	OutputRaw       []ConfigRaw `json:"output"`
}

// In/Out chan.
type InChan chan LogEvent
type OutChan chan LogEvent

// Set injector value.
func (c *CommonConfig) SetInjector(inj inject.Injector) {
	c.Injector = inj
}

// Get config type.
func (c *CommonConfig) GetType() string {
	return c.Type
}

func CheckErrorValues(refvs []reflect.Value) (err error) {
	for _, refv := range refvs {
		if refv.IsValid() {
			refvi := refv.Interface()
			switch refvi.(type) {
			case error:
				return refvi.(error)
			}
		}
	}
	return
}

// Invoke all reflect-values.
func (c *CommonConfig) Invoke(f interface{}) (refvs []reflect.Value, err error) {
	// return inject.Invoker(c.Injector, f)
	if refvs, err = c.Injector.Invoke(f); err != nil {
		return
	}
	err = CheckErrorValues(refvs)
	return
}

// Load config from file.
func LoadFromFile(path string) (config Config, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	return LoadFromData(data)
}

// Laod config from string.
func LoadFromString(text string) (config Config, err error) {
	return LoadFromData([]byte(text))
}

// Laod default-config from string.
func LoadDefaultConfig() (config Config, err error) {
	return LoadFromString(Defaultconfig)
}

// Load config from data([]byte).
func LoadFromData(data []byte) (config Config, err error) {
	if data, err = CleanComments(data); err != nil {
		return
	}

	if err = json.Unmarshal(data, &config); err != nil {
		glog.Errorln(err)
		return
	}

	config.Injector = inject.New()
	config.Map(Logger)

	inchan := make(InChan, 100)
	outchan := make(OutChan, 100)
	config.Map(inchan)
	config.Map(outchan)

	rv := reflect.ValueOf(&config)
	formatReflect(rv)

	return
}

// Reflect config.
func ReflectConfig(confraw *ConfigRaw, conf interface{}) (err error) {
	data, err := json.Marshal(confraw)
	if err != nil {
		return
	}

	if err = json.Unmarshal(data, conf); err != nil {
		return
	}

	rv := reflect.ValueOf(conf).Elem()
	formatReflect(rv)

	return
}

// Format reflect.
func formatReflect(rv reflect.Value) {
	if !rv.IsValid() {
		return
	}

	switch rv.Kind() {
	case reflect.Ptr:
		if !rv.IsNil() {
			formatReflect(rv.Elem())
		}
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Field(i)
			formatReflect(field)
		}
	case reflect.String:
		if !rv.CanSet() {
			return
		}
		value := rv.Interface().(string)
		value = FormatWithEnv(value)
		rv.SetString(value)
	}
}

// CleanComments used for remove non-standard json comments.
// Supported comment formats ^\s*# and ^\s*//
func CleanComments(data []byte) (out []byte, err error) {
	reForm1 := regexp.MustCompile(`^\s*#`)
	reForm2 := regexp.MustCompile(`^\s*//`)
	data = bytes.Replace(data, []byte("\r"), []byte(""), 0) // Windows
	lines := bytes.Split(data, []byte("\n"))
	var filtered [][]byte

	for _, line := range lines {
		if reForm1.Match(line) {
			continue
		}
		if reForm2.Match(line) {
			continue
		}
		filtered = append(filtered, line)
	}

	out = bytes.Join(filtered, []byte("\n"))
	return
}

// Simple-invoke.
func (c *Config) InvokeSimple(arg interface{}) (err error) {
	refvs, err := c.Injector.Invoke(arg)
	if err != nil {
		return
	}
	err = CheckErrorValues(refvs)
	return
}
