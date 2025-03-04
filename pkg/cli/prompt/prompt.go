package prompt

import "github.com/AlecAivazis/survey/v2"

// SurveyAskOne ask one question to be stubbed later.
var SurveyAskOne func(survey.Prompt, any, ...survey.AskOpt) error = survey.AskOne

// SurveyAsk ask questions to be stubbed later.
var SurveyAsk func([]*survey.Question, any, ...survey.AskOpt) error = survey.Ask
