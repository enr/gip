package environment

import (
	"os"
	"runtime"
	"testing"
)

type envvar struct {
	key   string
	value string
}

var vars = []envvar{
	{"TestGetenvEitherCase", "TestGetenvEitherCase_camel"},
	{"TESTGETENVEITHERCASE", "TestGetenvEitherCase_upper"},
	{"testgetenveithercase", "TestGetenvEitherCase_lower"},
}

func TestGetenvEitherCase(t *testing.T) {
	for _, env := range vars {
		os.Setenv(env.key, env.value)
		res := GetenvEitherCase("TestGetenvEitherCase")
		if res != env.value {
			t.Errorf(`Env %s, got %s, expected %s`, env.key, res, env.value)
		}
		os.Setenv(env.key, "")
	}
}

// No on windows
func TestGetenvEitherCaseAllTogether(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	for _, env := range vars {
		os.Setenv(env.key, env.value)
	}
	for _, env := range vars {
		res := GetenvEitherCase(env.key)
		if res != env.value {
			t.Errorf(`Env %s, got %s, expected %s`, env.key, res, env.value)
		}
	}
	for _, env := range vars {
		os.Setenv(env.key, "")
	}
}

func TestGetenvEitherCase_emptykey(t *testing.T) {
	res := GetenvEitherCase("")
	if res != "" {
		t.Errorf(`Env "", got "%s", expected ""`, res)
	}
}

func TestWhich_smoke(t *testing.T) {
	res := Which("go")
	if res == "" {
		t.Errorf(`Which(go), got "", expected something...`)
	}
}

func TestWhichFullPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	res := Which("/bin/bash")
	if res != "/bin/bash" {
		t.Errorf(`TestWhichFullPath, expected /bin/bash but got "%s"`, res)
	}
}
