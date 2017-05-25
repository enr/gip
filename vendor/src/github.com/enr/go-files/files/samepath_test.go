package files

import (
	"testing"
)

type samepathTestCase struct {
	p1 string
  p2 string
	equals bool
}

var testcases = []samepathTestCase{
	{"", " ", true},
	{".notfound", "../files/.notfound", true},
	{".notfound", `..\files\.notfound`, true},
	{".", "../files", true},
	{"testdata/", "./testdata", true},
}

func TestSamePath(t *testing.T) {
	for _, data := range testcases {
		res := IsSamePath(data.p1, data.p2)
		if res != data.equals {
			t.Errorf(`Expected IsSamePath=%t for paths "%s" and "%s"`, data.equals, data.p1, data.p2)
		}
	}
}
