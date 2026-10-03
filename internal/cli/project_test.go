package cli_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
)

func load(t *testing.T, root string) *project.Project {
	t.Helper()
	p, err := project.Load(background, root, "pkl", proc.Run)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPklInitLinkedProjectLoads(t *testing.T) {
	root := newProject(t, "my-map")
	for _, name := range []string{"moonwell.pkl", "moonwell.local.pkl", "src/main.yue", "maps/map.w3x/war3map.lua", "PklProject.deps.json", ".gitignore", ".gitattributes"} {
		if !exists(root, name) {
			t.Error(name)
		}
	}
	if exists(root, "deno.json") {
		t.Fatal("init wrote deno.json")
	}
	p := load(t, root)
	if p.Map.Folder != "map.w3x" || p.Launch.GameExecutable == nil || *p.Launch.GameExecutable != project.DefaultGameExecutable || !reflect.DeepEqual(p.Launch.Args, []string{"-launch", "-windowmode", "windowed"}) {
		t.Fatalf("%+v", p)
	}
	for _, folder := range []string{"CommandButtons", "CommandButtonsDisabled", "PassiveButtons"} {
		if !exists(root, "assets/ReplaceableTextures/"+folder) {
			t.Error(folder)
		}
	}
	env, _ := newEnv(root)
	plan, err := cli.Assets(background, env, false)
	if err != nil || len(plan.Assets) != 0 {
		t.Fatalf("%v %+v", err, plan)
	}
}

func TestPklInitRefusesNonemptyDirectory(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "keep.txt", "keep")
	env, _ := newEnv(filepath.Dir(root))
	_, err := cli.Init(background, env, root, cli.InitOptions{Link: true})
	contains(t, asError(t, err, "init").Msg, "not empty")
	if read(t, root, "keep.txt") != "keep" {
		t.Fatal("init changed file")
	}
}
