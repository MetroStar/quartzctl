package config

import (
	"net/url"
	"reflect"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

const redacted = "[REDACTED]"

// RedactedYAML preserves known configuration fields, masking credentials and
// opaque extensions rather than assuming unknown values are safe to publish.
func RedactedYAML(k *koanf.Koanf) ([]byte, error) {
	return yaml.Parser().Marshal(redactConfigStruct(k.Raw(), reflect.TypeFor[schema.QuartzConfig]()))
}

func redactConfig(value interface{}, typ reflect.Type) interface{} {
	if value == nil {
		return nil
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		values, ok := value.(map[string]interface{})
		if !ok {
			return redacted
		}
		return redactConfigStruct(values, typ)
	case reflect.Map:
		values, ok := value.(map[string]interface{})
		if !ok || typ.Key().Kind() != reflect.String {
			return redacted
		}
		out := make(map[string]interface{}, len(values))
		for key, child := range values {
			if sensitiveConfigField(key) || typ.Elem().Kind() != reflect.Struct {
				out[key] = redacted
			} else {
				out[key] = redactConfig(child, typ.Elem())
			}
		}
		return out
	case reflect.Slice, reflect.Array:
		values := reflect.ValueOf(value)
		if values.Kind() != reflect.Slice && values.Kind() != reflect.Array {
			return redacted
		}
		out := make([]interface{}, values.Len())
		for i := range out {
			out[i] = redactConfig(values.Index(i).Interface(), typ.Elem())
		}
		return out
	case reflect.String:
		text, ok := value.(string)
		if !ok {
			return redacted
		}
		if strings.Contains(text, "://") || strings.ContainsAny(text, "?#") {
			u, err := url.Parse(text)
			if err != nil || u.Opaque != "" || (u.Scheme != "" && u.Host == "") {
				return redacted
			}
			u.User = nil
			u.RawQuery = ""
			u.ForceQuery = false
			u.Fragment = ""
			u.RawFragment = ""
			return u.String()
		}
		return text
	case reflect.Bool:
		if _, ok := value.(bool); ok {
			return value
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch reflect.TypeOf(value).Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return value
		}
	}
	return redacted
}

func redactConfigStruct(values map[string]interface{}, typ reflect.Type) map[string]interface{} {
	fields := make(map[string]reflect.Type)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		key := field.Tag.Get("koanf")
		if field.IsExported() && key != "" && key != "-" && key != "__internal__" {
			fields[key] = field.Type
		}
	}
	out := make(map[string]interface{}, len(values))
	for key, child := range values {
		fieldType, known := fields[key]
		switch {
		case !known || sensitiveConfigField(key):
			out[key] = redacted
		case typ == reflect.TypeFor[schema.ApplicationRepositoryConfig]() && key == "settings":
			out[key] = redactApplicationSettings(child)
		default:
			out[key] = redactConfig(child, fieldType)
		}
	}
	return out
}

func sensitiveConfigField(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	for _, part := range []string{"password", "passwd", "secret", "token", "credential", "privatekey", "apikey", "accesskey", "authorization", "cookie", "headers"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return key == "value"
}

func redactApplicationSettings(value interface{}) interface{} {
	values, ok := value.(map[string]interface{})
	if !ok {
		return redacted
	}
	out := make(map[string]interface{}, len(values))
	for key, child := range values {
		out[key] = redacted
		if key == "post_deploy" {
			if enabled, ok := child.(bool); ok {
				out[key] = enabled
			} else {
				out[key] = redactConfig(child, reflect.TypeFor[schema.ApplicationPostDeployConfig]())
			}
		}
	}
	return out
}
