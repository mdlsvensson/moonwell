# Moonwell

A framework for making Warcraft III maps in C#. A map is one C# project: units, abilities, buffs and items are
declared as classes based on the game's own objects, their behaviour is made of components, and the build turns the
project and a World Editor map into a playable map, with the script compiled to Lua.

`dotnet new install Moonwell.Templates` adds `dotnet new moonwell`, which makes a new map project; `docs/guide.md` shows
how to start one and what goes in it.

`moonwell build [folder] [--release]` builds the map project in the folder (the current one by default) into
`build\<Name>.w3x`. A development build is the default; `--release` makes a release build.
`moonwell run [folder] [--release] [--game <exe>]` builds the same way, then starts Warcraft III with the map.

An object is a class in the project that inherits one of the standard classes of `Moonwell.Objects` (`class Arthas : Paladin`);
its constructor changes the fields, and the build writes the object into the map and keeps its id in `ids.json`. Set
`DataSet` and `DataVersion` in the map class, as World Editor's Data Set and Data Version, to play with another balance.

An error in a reaction is shown on the screen with the C# file and line it happened at, and written in full to
`Documents\Warcraft III\CustomMapData\Moonwell\log.txt` with the lines that called it; a release build shows no message
on the screen. Expression-bodied members and field initializers name no line of their own, so the line that used them
is shown.

It needs the .NET 10 SDK, and `common.j` and `Blizzard.j` in `Documents\Warcraft III\JassHelper`.

| Path | What it is |
|---|---|
| `src/Moonwell.Natives` | The game's natives as C#, generated |
| `src/Moonwell` | The runtime: base types, components, events, ownership |
| `src/Moonwell.Objects` | The game's standard objects as classes to inherit from, generated |
| `src/Moonwell.Build` | The build: reads a map project, writes the map |
| `src/Moonwell.Cli` | The `moonwell` command |
| `template/` | The map project template, `dotnet new moonwell` |
| `docs/guide.md` | How to make a map |
| `generator/` | Makes `Moonwell.Natives` and `Moonwell.Objects` from the game's data |
| `tests/` | Tests |
