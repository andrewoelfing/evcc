package configure

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util/templates"
	"github.com/thoas/go-funk"
)

// surveyAskOne asks the user for input
func (c *CmdConfigure) surveyAskOne(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	opts = append(opts, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = ""
	}))
	err := survey.AskOne(p, response, opts...)

	if err != nil {
		if err == terminal.InterruptErr {
			fmt.Println(c.localizedString("Cancel", nil))
			os.Exit(0)
		}
		fmt.Printf("%s %s\n", c.localizedString("InputError", nil), err)
	}

	return err
}

// askConfigFailureNextStep asks the user if he/she wants to select another device because the current does not work, or continue
func (c *CmdConfigure) askConfigFailureNextStep() bool {
	fmt.Println()
	return c.askYesNo(c.localizedString("TestingDevice_RepeatStep", nil))
}

// select item from list
func (c *CmdConfigure) askSelection(message string, items []string) (error, string, int) {
	selection := ""
	prompt := &survey.Select{
		Message: message,
		Options: items,
	}

	err := c.surveyAskOne(prompt, &selection)
	if err != nil {
		return err, "", 0
	}

	var selectedIndex int
	for index, item := range items {
		if item == selection {
			selectedIndex = index
			break
		}
	}

	return err, selection, selectedIndex
}

// selectItem selects item from list
func (c *CmdConfigure) selectItem(deviceCategory DeviceCategory) templates.Template {
	var emptyItem templates.Template
	emptyItem.Description = c.localizedString("ItemNotPresent", nil)

	elements := c.fetchElements(deviceCategory)
	elements = append(elements, emptyItem)

	var items []string
	for _, item := range elements {
		if item.Description != "" {
			items = append(items, item.Description)
		}
	}

	text := fmt.Sprintf("%s %s %s:", c.localizedString("Choose", nil), DeviceCategories[deviceCategory].article, DeviceCategories[deviceCategory].title)
	err, _, selected := c.askSelection(text, items)
	if err != nil {
		c.log.FATAL.Fatal(err)
	}

	return elements[selected]
}

// askChoice selects item from list
func (c *CmdConfigure) askChoice(label string, choices []string) (int, string) {
	err, selection, index := c.askSelection(label, choices)
	if err != nil {
		c.log.FATAL.Fatal(err)
	}

	return index, selection
}

// askYesNo asks yes/no question, return true if yes is selected
func (c *CmdConfigure) askYesNo(label string) bool {
	confirmation := false
	prompt := &survey.Confirm{
		Message: label,
	}

	err := c.surveyAskOne(prompt, &confirmation)
	if err != nil {
		c.log.FATAL.Fatal(err)
	}

	return confirmation
}

type question struct {
	label, help                    string
	defaultValue, exampleValue     interface{}
	invalidValues                  []string
	valueType                      string
	minNumberValue, maxNumberValue int64
	mask, required                 bool
	excludeNone                    bool
}

// askBoolValue asks for a boolean value selection for a given question
func (c *CmdConfigure) askBoolValue(label string) string {
	choices := []string{c.localizedString("Config_No", nil), c.localizedString("Config_Yes", nil)}
	values := []string{"false", "true"}

	index, _ := c.askChoice(label, choices)
	return values[index]
}

// askValue asks for value input for a given question (template param)
func (c *CmdConfigure) askValue(q question) string {
	if q.valueType == templates.ParamValueTypeBool {
		label := q.label
		if q.help != "" {
			label = q.help
		}

		return c.askBoolValue(label)
	}

	if q.valueType == templates.ParamValueTypeChargeModes {
		chargingModes := []string{string(api.ModeOff), string(api.ModeNow), string(api.ModeMinPV), string(api.ModePV)}
		chargeModes := []string{
			c.localizedString("ChargeModeOff", nil),
			c.localizedString("ChargeModeNow", nil),
			c.localizedString("ChargeModeMinPV", nil),
			c.localizedString("ChargeModePV", nil),
		}
		if !q.excludeNone {
			chargingModes = append(chargingModes, "")
			chargeModes = append(chargeModes, c.localizedString("ChargeModeNone", nil))
		}
		modeChoice, _ := c.askChoice(c.localizedString("ChargeMode_Question", nil), chargeModes)
		return chargingModes[modeChoice]
	}

	input := ""

	var err error

	validate := func(val interface{}) error {
		value := val.(string)
		if q.invalidValues != nil && funk.ContainsString(q.invalidValues, value) {
			return errors.New(c.localizedString("ValueError_Used", nil))
		}

		if q.required && len(value) == 0 {
			return errors.New(c.localizedString("ValueError_Empty", nil))
		}

		if q.valueType == templates.ParamValueTypeFloat {
			_, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return errors.New(c.localizedString("ValueError_Float", nil))
			}
		}

		if q.valueType == templates.ParamValueTypeNumber {
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return errors.New(c.localizedString("ValueError_Number", nil))
			}
			if q.minNumberValue != 0 && intValue < q.minNumberValue {
				return errors.New(c.localizedString("ValueError_NumberLowerThanMin", localizeMap{"Min": q.minNumberValue}))
			}
			if q.maxNumberValue != 0 && intValue > q.maxNumberValue {
				return errors.New(c.localizedString("ValueError_NumberBiggerThanMax", localizeMap{"Max": q.maxNumberValue}))
			}
		}

		return nil
	}

	help := q.help
	if q.required {
		help += " (" + c.localizedString("Value_Required", nil) + ")"
	} else {
		help += " (" + c.localizedString("Value_Optional", nil) + ")"
	}
	if q.exampleValue != nil && q.exampleValue != "" {
		help += fmt.Sprintf(" ("+c.localizedString("Value_Sample", nil)+": %s)", q.exampleValue)
	}

	if q.mask {
		prompt := &survey.Password{
			Message: q.label,
			Help:    help,
		}
		err = c.surveyAskOne(prompt, &input, survey.WithValidator(validate))

	} else {
		prompt := &survey.Input{
			Message: q.label,
			Help:    help,
		}
		if q.defaultValue != nil {
			switch q.defaultValue.(type) {
			case string:
				prompt.Default = q.defaultValue.(string)
			case int:
				prompt.Default = strconv.Itoa(q.defaultValue.(int))
			case bool:
				prompt.Default = strconv.FormatBool(q.defaultValue.(bool))
			}
		}
		err = c.surveyAskOne(prompt, &input, survey.WithValidator(validate))
	}

	if err != nil {
		c.log.FATAL.Fatal(err)
	}

	return input
}