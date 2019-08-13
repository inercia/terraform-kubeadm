// Copyright © 2019 Alvaro Saurin
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

package ssh

import (
	"os"
	"testing"
)

func TestTempFilenames(t *testing.T) {
	name1, err := GetTempFilename()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	name2, err := GetTempFilename()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	names := []struct {
		name string
		res  bool
	}{
		{
			name: name1,
			res:  true,
		},
		{
			name: name2,
			res:  true,
		},
		{
			name: "/tmp/something.tmp",
			res:  false,
		},
	}

	for _, testCase := range names {
		isTemp := IsTempFilename(testCase.name)
		if isTemp != testCase.res {
			t.Fatalf("Error: %q detected as temp=%t when we expected temp=%t", testCase.name, isTemp, testCase.res)
		}
	}
}

func TestCheckLocalFileExists(t *testing.T) {
	ctx := NewTestingContext()

	name1, err := GetTempFilename()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	defer func() {
		DoTry(DoDeleteLocalFile(name1)).Apply(ctx)
	}()

	f, err := os.Create(name1)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	_, err = f.Write([]byte("something"))
	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	exists, err := CheckLocalFileExists(name1).Check(ctx)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	if !exists {
		t.Fatalf("Error: unexpected result for exists: %t", exists)
	}
}

func TestCheckFileExists(t *testing.T) {
	ctx := NewTestingContext()
	name1, err := GetTempFilename()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	// return a CONDITION_SUCCEEDED
	ctx = NewTestingContextWithResponses([]string{"CONDITION_SUCCEEDED"})
	exists, err := CheckFileExists(name1).Check(ctx)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	if !exists {
		t.Fatalf("Error: unexpected result for exists: %t", exists)
	}
}

func TestDoUploadReaderToFile(t *testing.T) {
	ctx, uploads := NewTestingContextForUploads([]string{})

	dst := "/tmp/something.txt"
	s := "this is a test"

	actions := ActionList{
		DoUploadBytesToFile([]byte(s), dst),
	}
	if res := actions.Apply(ctx); IsError(res) {
		t.Fatalf("Error: when running actions: %s", res)
	}
	if len(*uploads) == 0 {
		t.Fatalf("Error: upload not found in %+v", uploads)
	}
}

func TestLeftovers(t *testing.T) {
	ctx := NewTestingContextWithResponses([]string{})

	actions := ActionList{
		DoAddLeftover("/tmp/test1"),
		DoAddLeftover("/tmp/test2"),
		DoCleanupLeftovers(),
	}
	if res := actions.Apply(ctx); IsError(res) {
		t.Fatalf("Error: when running actions: %s", res)
	}
}
