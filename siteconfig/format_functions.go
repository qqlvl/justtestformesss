package siteconfig

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type FormatterType string

const (
	formatterTypeTitle    FormatterType = "title"
	formatterTypeUpper    FormatterType = "upper"
	formatterTypeLeftPad  FormatterType = "leftpad"
	formatterTypeRightPad FormatterType = "rightpad"
)

type formatter interface {
	Format(val string, args []string) (string, error)
	Type() FormatterType
}

type FormatterTitle struct{}

func (f FormatterTitle) Format(val string, args []string) (string, error) {
	return strings.Title(val), nil
}

func (f FormatterTitle) Type() FormatterType {
	return formatterTypeTitle
}

type FormatterUpper struct{}

func (f FormatterUpper) Format(val string, args []string) (string, error) {
	return strings.ToUpper(val), nil
}

func (f FormatterUpper) Type() FormatterType {
	return formatterTypeUpper
}

func readAnIntFromArgsOrReturnError(args []string) (int, error) {
	if len(args) == 0 {
		return 0, errors.New("no padding length provided")
	}

	return strconv.Atoi(args[0])
}

type FormatterLeftPad struct{}

func (f FormatterLeftPad) Format(val string, args []string) (string, error) {
	cnt, err := readAnIntFromArgsOrReturnError(args)
	if err != nil {
		return "", err
	}

	for len(val) < cnt {
		val = " " + val
	}

	return val, nil
}

func (f FormatterLeftPad) Type() FormatterType {
	return formatterTypeLeftPad
}

type FormatterRightPad struct{}

func (f FormatterRightPad) Format(val string, args []string) (string, error) {
	cnt, err := readAnIntFromArgsOrReturnError(args)
	if err != nil {
		return "", err
	}

	for len(val) < cnt {
		val = val + " "
	}

	return val, nil
}

func (f FormatterRightPad) Type() FormatterType {
	return formatterTypeRightPad
}

func formatWithFormatters(val string, formatters []string) (string, error) {
	var f formatter
	var args []string
	var err error

	for _, formatterName := range formatters {
		// extract args. If they exist, they will be in the form of (arg1, arg2, arg3), like
		// leftpad(5)
		if strings.Contains(formatterName, "(") {
			start := strings.Index(formatterName, "(")
			end := strings.Index(formatterName, ")")
			args = strings.Split(formatterName[start+1:end], ",")
			formatterName = formatterName[:start]
		}

		switch FormatterType(formatterName) {
		case formatterTypeTitle:
			f = FormatterTitle{}
		case formatterTypeUpper:
			f = FormatterUpper{}
		case formatterTypeLeftPad:
			f = FormatterLeftPad{}
		case formatterTypeRightPad:
			f = FormatterRightPad{}
		default:
			return "", fmt.Errorf("unknown formatter: %s", formatterName)
		}
		val, err = f.Format(val, args)
		if err != nil {
			return "", err
		}
	}

	return val, nil
}
