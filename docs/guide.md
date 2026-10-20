# Making a map with Moonwell

## Start a map

Moonwell needs the .NET 10 SDK, Warcraft III 3.0, and `common.j` and `Blizzard.j` in
`Documents\Warcraft III\JassHelper`. Install the template and the `moonwell` command once:

```
dotnet new install Moonwell.Templates
dotnet tool install --global Moonwell.Cli
```

Then make a map and play it:

```
dotnet new moonwell -n MyMap
cd MyMap
moonwell run
```

The game opens the map. Player 1 has a Champion, a Paladin of the map's own, with three footmen to fight. Learn Smite
and cast it: the screen names the target. Each kill shows the Champion's count.

## The project

| Path | What it is |
|---|---|
| `MyMap.csproj` | The C# project; the built map takes its name |
| `Map.cs` | The map's settings, and the components the game starts with |
| `Start.cs` | The game's start: makes the Champion and the footmen |
| `Champion/` | One feature: the Champion, its spell and its component |
| `map/` | The World Editor map, saved as a folder |
| `assets/` | Files copied into the map at the same paths (models, icons, sounds); make it when you need it |
| `ids.json` | The object ids the build gave, written by the build; commit it |
| `build/` | What the build writes: the staged map and `MyMap.w3x`; do not commit it |

Code can sit in any folder: the build finds objects and the map class by what they inherit. Keep one folder per
feature.

World Editor is for terrain and placed things: regions, cameras, units, items, trees. Open `map/` in World Editor,
change it, and save it as a folder again. Objects, triggers and the map's settings are in code; the build replaces the
saved script.

## The map's settings

`Map.cs` holds the one class of the project that inherits `Map`. Its constructor sets `Name`, `Author`, `Description`
and `RecommendedPlayers`; one left empty keeps World Editor's value. `DataSet` and `DataVersion` pick the balance the
map plays with, as World Editor's Data Set and Data Version. `Components.Add` lists the game components the game
starts with.

## Objects

A unit, hero, item, ability, buff, destructable, doodad or upgrade is a class that inherits one of the game's own,
from `Moonwell.Objects`: `Units.Human.Paladin`, `Abilities.Human.StormBolt`, `Items.ClawsOfAttack15`. Its constructor
changes fields; a field it does not change keeps its base's value.

```csharp
public sealed class Champion : Paladin
{
    public Champion()
    {
        Name = "Champion";
        HitPoints = HitPoints + 200;
        HeroAbilities.Remove<HolyLight>();
        HeroAbilities.Add<Smite>();
        Components.Add(new Fury());
    }
}
```

- A field with levels takes `Levels(75f, 150f, 225f)`, or `PerLevel(level => ...)` to compute each level.
- A field with no name of its own is set by its id: `Set("Htb1", 2, 250f)`.
- A constructor may compute, but must not call the game: the build fails at such a line.
- The build gives each object an id and keeps it in `ids.json`. `[ObjectId("H001")]` on the class pins one.
- `[Replaces]` makes the class the game's object itself: every unit of that type is made as the class.
- `Unit.Create<Champion>(player, x, y, facing)` makes one in the game.

## Components and reactions

A reaction is a method on the thing involved: `OnDamaged`, `OnDeath` and `OnKill` on a unit, `OnCast` on an ability,
`OnPickedUp` on an item. A component is a class that carries reactions and state, and is added to a unit, item,
destructable, player or the game: it inherits `UnitComponent`, `ItemComponent`, `DestructableComponent`,
`PlayerComponent` or `GameComponent`.

```csharp
public sealed class Fury : UnitComponent
{
    private int kills;

    public override void OnKill(Unit victim)
    {
        kills++;
        Natives.DisplayTextToPlayer(Unit.Player.Handle, 0f, 0f, "Kills: " + kills);
    }
}
```

- A type lists the components its instances start with in its constructor. `unit.Add(new Fury())` adds one in the
  game, and `End()` removes it.
- Reactions to the whole map (`OnGameStart`, `OnUnitDied`, `OnUnitCast`) go on a game or player component.
- An event runs the type's reaction first, then its components' in the order added, then the game's and the
  players'. A reaction that fails is reported, and the others still run.
- A buff is a class that inherits `Buff`, or a standard buff to show its icon. `Duration` and `Tick` set its time,
  `OnApply`, `OnTick` and `OnExpire` are its reactions, and `unit.Apply(new Burn())` puts it on a unit.
- `Timer.After(seconds, action)` and `Timer.Every(seconds, action)` run code later. A timer, buff or component
  belongs to whatever made it, and ends with it.
- Game work starts in a reaction. Top-level code and static fields make nothing in the game.

## Players

`Player.One` to `Player.TwentyFour` are the player slots, as the lobby numbers them; `Player.Get(slot)` takes the
slot from 0, so `Player.Get(0)` is `Player.One`. A slot is not a colour: a map can recolour its players.
`Player.NeutralAggressive`, `NeutralPassive`, `NeutralVictim` and `NeutralExtra` are the neutral players, and
`StartX` and `StartY` a player's start location.

The `Natives` class has every native of the game for what Moonwell does not cover yet.

## Multiplayer

Code that differs between players' machines goes in `Local.For(player, () => ...)`, as Smite reads a unit's name,
which is in each player's language. The build rejects, as compile
errors, a change to the game inside it, a machine-local read outside it, `GetHandleId`, looping over a `Dictionary` or
`HashSet` (use `OrderedMap` and `OrderedSet`), and `long`, `ulong`, `uint` and `double`. `[SyncUnchecked]` on a method
turns the checks off in it.

## The build

`moonwell build` builds the map in the current folder into `build\MyMap.w3x`; `moonwell run` builds it and starts the
game with it. Both make a development build: it checks the values Moonwell receives and keeps the Lua readable.
`--release` makes the release build. `moonwell run --game <exe>` names the game when it is not in its usual place.

An error in a reaction shows on the screen with its C# file and line, and is written in full to
`Documents\Warcraft III\CustomMapData\Moonwell\log.txt`. A release build leaves the message off the screen and keeps
the file and line.

To place objects declared in code in World Editor, save `build\MyMap.w3x` from World Editor as the `map/` folder. The
next build keeps the placed units; code wins over any object it also declares.
