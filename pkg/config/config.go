package config

import (
	"reflect"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Load reads configuration from .env files, config files, and environment variables into a struct.
// It supports "mapstructure" tags (standard for viper) and "env" tags (legacy support).
func Load(target interface{}) error {
	// 1. Load .env file (optional)
	// We ignore the error here because .env file might not exist, which is fine.
	_ = godotenv.Load()

	// 2. Initialize Viper
	v := viper.New()
	v.AutomaticEnv() // Read from environment variables

	// 3. Map "env" tags to Viper keys for backward compatibility
	val := reflect.ValueOf(target)
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		elem := val.Elem()
		if elem.Kind() == reflect.Struct {
			bindViperTags(v, elem)
		}
	}

	// 4. Set config file search paths
	v.SetConfigName("config")
	v.SetConfigType("yaml") // Default to yaml if no extension
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// 5. Read Config File (optional)
	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file is not found
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	// 6. Unmarshal
	return v.Unmarshal(target, func(dc *mapstructure.DecoderConfig) {
		dc.Squash = true
	})
}

func bindViperTags(v *viper.Viper, elem reflect.Value) {
	typ := elem.Type()
	for i := 0; i < elem.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Anonymous {
			fieldVal := elem.Field(i)
			if fieldVal.Kind() == reflect.Ptr && !fieldVal.IsNil() {
				fieldVal = fieldVal.Elem()
			}
			if fieldVal.Kind() == reflect.Struct {
				bindViperTags(v, fieldVal)
				continue
			}
		}

		key := field.Tag.Get("mapstructure")
		if key == "" {
			key = field.Name
		}

		// Handle "env" tag
		tag := field.Tag.Get("env")
		if tag != "" {
			_ = v.BindEnv(key, tag)
		}

		// Handle "default" tag
		def := field.Tag.Get("default")
		if def != "" {
			v.SetDefault(key, def)
		}
	}
}
