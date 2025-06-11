package siteconfig

import (
	"regexp"
	"strings"
)

var fmtRegex = regexp.MustCompile(`\$\{([a-zA-Z0-9_:()]+)}`)

func formatString(s string, dictionary map[string]string) (string, error) {
	formatPieces := fmtRegex.FindAllStringSubmatch(s, -1)
	for _, pieces := range formatPieces {
		split := strings.Split(pieces[1], ":")
		variable := split[0]
		if _, ok := dictionary[variable]; !ok {
			continue
		}
		formatters := split[1:]
		if len(formatters) == 0 {
			s = strings.ReplaceAll(s, pieces[0], dictionary[variable])
		} else {
			formatted, err := formatWithFormatters(dictionary[variable], formatters)
			if err != nil {
				return "", err
			}
			s = strings.ReplaceAll(s, pieces[0], formatted)
		}
	}

	return s, nil
}
