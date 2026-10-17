# Moonwell

A framework for making Warcraft III maps in C#. A map is one C# project: units, abilities, buffs and items are
declared as classes based on the game's own objects, their behaviour is made of components, and the build turns the
project and a World Editor map into a playable map, with the script compiled to Lua.

Nothing works yet.

| Path | What it is |
|---|---|
| `src/Moonwell.Natives` | The game's natives as C#, generated |
| `src/Moonwell` | The runtime: base types, components, events, ownership |
| `src/Moonwell.Objects` | The game's standard objects as classes to inherit from, generated |
| `src/Moonwell.Build` | The build: reads a map project, writes the map |
| `src/Moonwell.Cli` | The `moonwell` command |
| `generator/` | Makes `Moonwell.Natives` and `Moonwell.Objects` from the game's data |
| `tests/` | Tests |
