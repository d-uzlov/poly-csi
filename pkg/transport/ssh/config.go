package ssh_wrapper

import (
	"fmt"
	"time"
)

type SshConfig struct {
	verified              bool
	ServerAddress         string        `yaml:"serverAddress"`
	Username              string        `yaml:"username"`
	DialTimeout           time.Duration `yaml:"dialTimeout"`
	DefaultCommandTimeout time.Duration `yaml:"defaultCommandTimeout"`
	InsecureEchoCommands  bool          `yaml:"insecureEchoCommands"`
	Auth                  []struct {
		Name          string `yaml:"name"`
		Type          string `yaml:"type"`
		ValueFromEnv  string `yaml:"valueFromEnv"`
		ValueFromFile string `yaml:"valueFromFile"`
	} `yaml:"auth"`
}

func (sc *SshConfig) Validate() (errorList []string, warningList []string) {
	if sc.ServerAddress == "" {
		errorList = append(errorList, "serverAddress: field is empty")
	}
	if sc.Username == "" {
		errorList = append(errorList, "username: field is empty")
	}
	if len(sc.Auth) == 0 {
		errorList = append(errorList, "auth: list is empty")
	}
	if sc.DialTimeout == 0 {
		warningList = append(warningList, "dialTimeout: empty, set to 1s as default")
		sc.DialTimeout = time.Second
	}
	if sc.DefaultCommandTimeout == 0 {
		warningList = append(warningList, "defaultCommandTimeout: empty, set to 1s as default")
		sc.DefaultCommandTimeout = time.Second
	}
	for i, a := range sc.Auth {
		if a.Name == "" {
			warningList = append(warningList, fmt.Sprintf("auth[i=%d].name: field is empty, set name to %d", i, i))
			a.Name = fmt.Sprintf("%d", i)
		}
		for j := range i {
			if sc.Auth[j].Name == a.Name {
				warningList = append(warningList, fmt.Sprintf("auth[i=%d].type: name '%s' is used several times, set name to %d", i, a.Name, i))
				a.Name = fmt.Sprintf("%d", i)
				break
			}
		}
		if a.Type == "" {
			errorList = append(errorList, fmt.Sprintf("auth[name=%s].type: field is empty", a.Name))
		}
		if a.Type != "password" && a.Type != "key" {
			errorList = append(errorList, fmt.Sprintf("auth[name=%s].type: type '%s' is not supported, must be one of: password, key", a.Name, a.Type))
		}
		if a.ValueFromEnv == "" && a.ValueFromFile == "" {
			errorList = append(errorList, fmt.Sprintf("auth[name=%s]: value fields are empty", a.Name))
		}
		if a.ValueFromEnv != "" && a.ValueFromFile != "" {
			errorList = append(errorList, fmt.Sprintf("auth[name=%s]: only one value field allowed", a.Name))
		}
	}

	sc.verified = true
	return
}
