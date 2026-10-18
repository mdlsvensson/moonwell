# Moonwell

A framework for making Warcraft III maps in C#. A map is one C# project: units, abilities, buffs and items are
declared as classes based on the game's own objects, their behaviour is made of components, and the build turns the
project and a World Editor map into a playable map, with the script compiled to Lua.

`moonwell build [folder] [--release]` builds the map project in the folder (the current one by default) into
`build\<Name>.w3x`. A development build is the default; `--release` makes a release build.
`moonwell run [folder] [--release] [--game <exe>]` builds the same way, then starts Warcraft III with the map.

It needs the .NET 10 SDK, and `common.j` and `Blizzard.j` in `Documents\Warcraft III\JassHelper`.

| Path | What it is |
|---|---|
| `src/Moonwell.Natives` | The game's natives as C#, generated |
| `src/Moonwell` | The runtime: base types, components, events, ownership |
| `src/Moonwell.Objects` | The game's standard objects as classes to inherit from, generated |
| `src/Moonwell.Build` | The build: reads a map project, writes the map |
| `src/Moonwell.Cli` | The `moonwell` command |
| `generator/` | Makes `Moonwell.Natives` and `Moonwell.Objects` from the game's data |
| `tests/` | Tests |
