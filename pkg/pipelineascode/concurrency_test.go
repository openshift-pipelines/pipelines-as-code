package pipelineascode

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExecutionOrder(t *testing.T) {
	cm := NewConcurrencyManager()

	testNs := "test"
	abcPR := &v1beta1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "abc", Namespace: testNs}}
	defPR := &v1beta1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "def", Namespace: testNs}}
	mnoPR := &v1beta1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "mno", Namespace: testNs}}
	pqrPR := &v1beta1.PipelineRun{ObjectMeta: metav1.ObjectMeta{Name: "pqr", Namespace: testNs}}

	cm.Enable()

	// add pipelineRuns in random order
	cm.AddPipelineRun(pqrPR)
	cm.AddPipelineRun(abcPR)
	cm.AddPipelineRun(mnoPR)
	cm.AddPipelineRun(defPR)

	order, runs := cm.GetExecutionOrder()
	assert.Equal(t, order, "test/abc,test/def,test/mno,test/pqr")
	assert.Equal(t, len(runs), 4)
}
