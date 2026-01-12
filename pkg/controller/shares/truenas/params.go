package truenas

type Commands struct {
	Midclt []string
}

func ParseCommands(src map[string][]string) (Commands, error) {
	midclt := src["midclt"]
	if len(midclt) == 0 {
		midclt = []string{"midclt"}
	}

	return Commands{
		Midclt: midclt,
	}, nil
}
