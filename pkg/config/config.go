package config

import (
	"reflect"

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
		typ := elem.Type()
		for i := 0; i < elem.NumField(); i++ {
			field := typ.Field(i)

			// Handle "env" tag
			tag := field.Tag.Get("env")
			if tag != "" {
				if err := v.BindEnv(field.Name, tag); err != nil {
					// In case of error binding, we just continue, but it's unlikely.
				}
			}

			// Handle "default" tag
			def := field.Tag.Get("default")
			if def != "" {
				v.SetDefault(field.Name, def)
			}
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
	return v.Unmarshal(target)
}
