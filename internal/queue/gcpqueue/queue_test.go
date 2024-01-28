// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gcpqueue

import (
	"testing"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/google/go-cmp/cmp"
	"github.com/khulnasoft-lab/godep/internal/config"
	"github.com/khulnasoft-lab/godep/internal/queue"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewTaskID(t *testing.T) {
	for _, test := range []struct {
		modulePath, version string
		want                string
	}{
		{"m-1", "v2", "acc5-m-1_vv2"},
		{"my_module", "v1.2.3", "0cb9-my__module_vv1_o2_o3"},
		{"µπΩ/github.com", "v2.3.4-ß", "a49c-_00b5_03c0_03a9_-github_ocom_vv2_o3_o4-_00df"},
	} {
		got := newTaskID(test.modulePath, test.version)
		if got != test.want {
			t.Errorf("%s@%s: got %s, want %s", test.modulePath, test.version, got, test.want)
		}
	}
}

func TestNewTaskRequest(t *testing.T) {
	cfg := config.Config{
		ProjectID:      "Project",
		LocationID:     "us-central1",
		QueueURL:       "http://1.2.3.4:8000",
		ServiceAccount: "sa",
		QueueAudience:  "qa",
	}
	want := &taskspb.CreateTaskRequest{
		Parent: "projects/Project/locations/us-central1/queues/queueID",
		Task: &taskspb.Task{
			DispatchDeadline: durationpb.New(maxCloudTasksTimeout),
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        "http://1.2.3.4:8000/fetch/mod/@v/v1.2.3",
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: "sa",
							Audience:            "qa",
						},
					},
				},
			},
		},
	}
	gcp, err := newGCP(&cfg, nil, "queueID")
	if err != nil {
		t.Fatal(err)
	}
	opts := &queue.Options{
		Suffix: "suf",
	}
	got := gcp.newTaskRequest("mod", "v1.2.3", opts)
	want.Task.Name = got.Task.Name
	if diff := cmp.Diff(want, got, cmp.Comparer(proto.Equal)); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	want.Task.MessageType.(*taskspb.Task_HttpRequest).HttpRequest.Url += "?proxyfetch=off"
	opts.DisableProxyFetch = true
	got = gcp.newTaskRequest("mod", "v1.2.3", opts)
	want.Task.Name = got.Task.Name
	if diff := cmp.Diff(want, got, cmp.Comparer(proto.Equal)); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

}
