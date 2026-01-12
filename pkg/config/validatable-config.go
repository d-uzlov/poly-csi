package config

type ValidatableConfig interface {
	// validate must be called before using the config
	// it checks that values in config are valid and compatible with each other
	// it can set default values, if required
	// when len(errorList) > 0 config is invalid and should not be used
	Validate() (errorList []string, warningList []string)
}
