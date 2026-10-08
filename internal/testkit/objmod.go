package testkit

import (
	"encoding/binary"
	"math"

	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

type SyntheticMod struct {
	Field  string
	Level  int32
	Column int32
	Value  objmod.Value
	End    string
}

type SyntheticSet struct {
	Flag int32
	Mods []SyntheticMod
}

type SyntheticObject struct {
	Base, ID string
	Sets     []SyntheticSet
	Mods     []SyntheticMod
}

func BuildModFile(version int32, original, custom []SyntheticObject, kind objmod.TableKind) []byte {
	var out []byte
	i32 := func(n int32) { out = binary.LittleEndian.AppendUint32(out, uint32(n)) }
	id := func(s string) {
		count := 0
		for _, r := range s {
			if r > 0xff {
				panic("test id must be 4 Latin-1 characters: " + s)
			}
			out = append(out, byte(r))
			count++
		}
		if count != 4 {
			panic("test id must be 4 Latin-1 characters: " + s)
		}
	}
	i32(version)
	for _, table := range [][]SyntheticObject{original, custom} {
		i32(int32(len(table)))
		for _, object := range table {
			id(object.Base)
			id(object.ID)
			sets := object.Sets
			if sets == nil {
				sets = []SyntheticSet{{Mods: object.Mods}}
			}
			if version >= 3 {
				i32(int32(len(sets)))
			} else if len(sets) != 1 {
				panic("v1/v2 objects have exactly one set")
			}
			for _, set := range sets {
				if version >= 3 {
					i32(set.Flag)
				}
				i32(int32(len(set.Mods)))
				for _, mod := range set.Mods {
					id(mod.Field)
					i32(int32(mod.Value.Type))
					if kind == objmod.Leveled {
						i32(mod.Level)
						i32(mod.Column)
					}
					switch mod.Value.Type {
					case objmod.String:
						out = append(append(out, mod.Value.Text...), 0)
					case objmod.Int:
						i32(mod.Value.Int)
					default:
						out = binary.LittleEndian.AppendUint32(out, math.Float32bits(mod.Value.Real))
					}
					if mod.End == "" {
						out = append(out, 0, 0, 0, 0)
					} else {
						id(mod.End)
					}
				}
			}
		}
	}
	return out
}
