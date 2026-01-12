package jsonfiledb

type JsonFileInfoManagerConfig struct {
	Directory string `yaml:"directory"`
}

func (c JsonFileInfoManagerConfig) Validate() (errorList []string, warningList []string) {
	if c.Directory == "" {
		errorList = append(errorList, "directory: field is empty")
	}
	return
}
