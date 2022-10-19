// Copyright © 2020 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package formatting

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapi "knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestTimeout(t *testing.T) {
	t1 := metav1.Duration{
		Duration: 5 * time.Minute,
	}

	str := Timeout(&t1) // Timeout is defined
	assert.Equal(t, str, "5 minutes")

	str = Timeout(nil) // Timeout is not defined
	assert.Equal(t, str, nonAttributedStr)
}

func TestAge(t *testing.T) {
	clock := clockwork.NewFakeClock()
	assert.Equal(t, Age(&metav1.Time{}, clock), nonAttributedStr)

	t1 := &metav1.Time{
		Time: clock.Now().Add(-5 * time.Minute),
	}
	assert.Equal(t, Age(t1, clock), "5 minutes ago")
}

func TestDuration(t *testing.T) {
	assert.Equal(t, Duration(&metav1.Time{}, &metav1.Time{}), nonAttributedStr)
	clock := clockwork.NewFakeClock()

	assert.Equal(t, Duration(&metav1.Time{
		Time: clock.Now(),
	}, &metav1.Time{
		Time: clock.Now().Add(5 * time.Minute),
	}), "5 minutes")
}

func TestPRDuration(t *testing.T) {
	clock := clockwork.NewFakeClock()
	infiveminutes := clock.Now().Add(time.Duration(5 * int(time.Minute)))
	type args struct {
		rr v1alpha1.RepositoryRunStatus
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no start time",
			args: args{},
			want: nonAttributedStr,
		},
		{
			name: "with completion time",
			args: args{
				rr: v1alpha1.RepositoryRunStatus{
					StartTime: &metav1.Time{
						Time: clock.Now(),
					},
					CompletionTime: &metav1.Time{
						Time: infiveminutes,
					},
				},
			},
			want: "5 minutes",
		},
		{
			name: "completion from first condition",
			args: args{
				rr: v1alpha1.RepositoryRunStatus{
					StartTime: &metav1.Time{
						Time: clock.Now(),
					},
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{
							{
								LastTransitionTime: knativeapi.VolatileTime{
									Inner: metav1.Time{Time: infiveminutes},
								},
							},
						},
					},
				},
			},
			want: "5 minutes",
		},
		{
			name: "with status but no conditions",
			args: args{
				rr: v1alpha1.RepositoryRunStatus{
					StartTime: &metav1.Time{
						Time: clock.Now(),
					},
					Status: duckv1beta1.Status{},
				},
			},
			want: nonAttributedStr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PRDuration(tt.args.rr); got != tt.want {
				t.Errorf("PRDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
