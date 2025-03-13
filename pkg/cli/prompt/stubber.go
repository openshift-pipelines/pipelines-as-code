package prompt

import (
	"fmt"
	"reflect"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
)

type AskStubber struct {
	Asks     [][]*survey.Question
	AskOnes  []*survey.Prompt
	Count    int
	OneCount int
	Stubs    [][]*QuestionStub
	StubOnes []*StubPrompt
}

func InitAskStubber() (*AskStubber, func()) {
	origSurveyAsk := SurveyAsk
	origSurveyAskOne := SurveyAskOne
	as := AskStubber{}

	SurveyAskOne = func(p survey.Prompt, response any, _ ...survey.AskOpt) error {
		as.AskOnes = append(as.AskOnes, &p)
		count := as.OneCount
		as.OneCount++
		if count >= len(as.StubOnes) {
			panic(fmt.Sprintf("more asks than stubs. most recent call: %v", p))
		}
		stubbedPrompt := as.StubOnes[count]
		if stubbedPrompt.Default {
			// TODO this is failing for basic AskOne invocations with a string result.
			defaultValue := reflect.ValueOf(p).Elem().FieldByName("Default")
			_ = core.WriteAnswer(response, "", defaultValue)
		} else {
			_ = core.WriteAnswer(response, "", stubbedPrompt.Value)
		}

		return nil
	}

	teardown := func() {
		SurveyAsk = origSurveyAsk
		SurveyAskOne = origSurveyAskOne
	}
	return &as, teardown
}

type StubPrompt struct {
	Value   any
	Default bool
}

type QuestionStub struct {
	Name    string
	Value   any
	Default bool
}

func (as *AskStubber) StubOne(value any) {
	as.StubOnes = append(as.StubOnes, &StubPrompt{
		Value: value,
	})
}

func (as *AskStubber) StubOneDefault() {
	as.StubOnes = append(as.StubOnes, &StubPrompt{
		Default: true,
	})
}

func (as *AskStubber) Stub(stubbedQuestions []*QuestionStub) {
	// A call to .Ask takes a list of questions; a stub is then a list of questions in the same order.
	as.Stubs = append(as.Stubs, stubbedQuestions)
}
