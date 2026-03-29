package transport

import "strings"

type ExecReplacement struct {
	commands map[string]commandReplacementConfig
}

type commandReplacementConfig struct {
	cmd    []string
	params commandParams
}

type commandParams struct {
}

func NewExecReplacement(cfg map[string][]string) *ExecReplacement {
	result := ExecReplacement{
		commands: map[string]commandReplacementConfig{},
	}
	for k, v := range cfg {
		cmdName, found := strings.CutSuffix(k, ":params")
		crc := result.commands[cmdName]
		if found {
			crc.params = parseCommandParams(v)
		} else {
			crc.cmd = v
		}
		result.commands[cmdName] = crc
	}
	return &result
}

func (c *ExecReplacement) Lookup(name string, preferSudo bool) []string {
	result := []string{name}
	crc, ok := c.commands[name]
	if ok && len(crc.cmd) != 0 {
		result = crc.cmd
	}
	if preferSudo {
		result = append([]string{"sudo"}, result...)
	}
	return result
}

func parseCommandParams(p []string) commandParams {
	result := commandParams{}
	for _, v := range p {
		// there are no params yet, this is just an idea how to implement them if needed
		switch v {
		}
	}
	return result
}
