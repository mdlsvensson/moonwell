package env

import (
	"reflect"
	"testing"
)

func TestNewFillsEveryField(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("MOONWELL_CACHE", cache)
	log := NewLogger(func(string) {}, "")
	env := New("the-project", log)
	if env.Root != "the-project" || env.Log != log || env.CacheDir != cache || env.Platform != CurrentPlatform() {
		t.Errorf("env = %+v", env)
	}
	if !sameFunc(env.Run, RunFunc(Run)) || !sameFunc(env.Spawn, SpawnDetached) {
		t.Errorf("Run or Spawn is not the real one: %+v", env)
	}
	// Platform is "" on a system without downloads; every other field has a value, also one added later.
	fields := reflect.ValueOf(*env)
	for i := range fields.NumField() {
		if name := fields.Type().Field(i).Name; name != "Platform" && fields.Field(i).IsZero() {
			t.Errorf("New left %s empty", name)
		}
	}
}

// sameFunc reports whether two function values are the same named function.
func sameFunc(a, b any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
