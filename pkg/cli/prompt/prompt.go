package prompt

import "github.com/AlecAivazis/survey/v2"

// SurveyAskOne ask one question to be stubbed later.
var SurveyAskOne = func(p survey.Prompt, response any, opts ...survey.AskOpt) error {
	return survey.AskOne(p, response, opts...)
}

// SurveyAsk ask questions to be stubbed later.
var SurveyAsk = func(qs []*survey.Question, response any, opts ...survey.AskOpt) error {
	return survey.Ask(qs, response, opts...)
}
