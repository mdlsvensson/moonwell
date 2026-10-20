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
| `Heroes/` | One feature: the Champion, its spell and its kill counter |
| `map/` | The World Editor map, saved as a folder |
| `assets/` | Files copied into the map at the same paths (models, icons, sounds); make it when you need it |
| `build/` | What the build writes: the staged map and `MyMap.w3x`; do not commit it |

Code can sit in any folder and any namespace: the build finds objects and the map class by what they inherit. The
template keeps one folder per feature, each with its own namespace (`MyMap.Heroes`).

World Editor is for terrain and placed things: regions, cameras, units, items, trees. Open `map/` in World Editor,
change it, and save it as a folder again. Objects, triggers and the map's settings are in code; the build replaces the
saved script.

## The map's settings

`Map.cs` holds the one class of the project that inherits `Map`. Its constructor sets `Name`, `Author`, `Description`
and `RecommendedPlayers`; one left empty keeps World Editor's value. `DataSet` and `DataVersion` pick the balance the
map plays with, as World Editor's Data Set and Data Version. `Components` lists the game components the game
starts with: `Components.Add(new Start(), new Rules());` adds them, and `Components = new() { new Start(), new Rules() };`
replaces the list.

## Objects

A unit, hero, item, ability, buff, destructable, doodad or upgrade is a class that inherits one of the game's own,
from `Moonwell.Objects`, by race and kind: `Human.Units.Paladin`, `Human.Abilities.Defend`, `Human.Upgrades.Defend`,
`Campaign.Units.Arthas`, and `Items.ClawsOfAttack15`, `Destructables.X` and `Doodads.X` for the kinds with no race.
`using Moonwell.Objects;` reaches all of them; `using static Moonwell.Objects.Human;` lets a file write
`Abilities.Defend`. Its constructor changes fields; a field it does not change keeps its base's value. Every new object
has its id on the class, as World Editor shows it: a hero's starts with an upper-case letter, another unit's with a
lower-case one.

```csharp
[ObjectId("H001")]
public sealed class Champion : Human.Units.Paladin
{
    public Champion()
    {
        Name = "Champion";
        HitPoints = HitPoints + 200;
        HeroAbilities.Replace<Human.Abilities.HolyLight, Smite>();
        Components.Add(new KillCounter());
    }
}
```

- A field with levels takes `Levels(75f, 150f, 225f)`, or `PerLevel(level => ...)` to compute each level.
- A field World Editor offers a list of values for holds an enum of them, named as the game's data spells them:
  `AttackType = AttackType.Magic`. One that takes several, such as the targets, holds flags:
  `Attack1TargetsAllowed = Targets.Air | Targets.Ground`.
- A list takes `Set<A, B, C>()` (the whole list, in order), `Replace<Old, New>()` (in the old one's place), `Add<T>()`
  and `Remove<T>()`.
- The tooltips are World Editor's four: `Tooltip` (Tooltip - Normal), `TooltipExtended` (Tooltip - Normal - Extended),
  `TooltipLearn` (Tooltip - Learn) and `TooltipLearnExtended` (Tooltip - Learn - Extended).
- A field with no name of its own is set by its id: `Set("Htb1", 2, 250f)`.
- A constructor may compute, but must not call the game: the build fails at such a line.
- A class with no `[ObjectId]` fails the build, which names a free id to use. A buff also names its carrier, the
  hidden ability that shows its icon: `[ObjectId("B001", Carrier = "A002")]`, or `[ObjectId(Carrier = "A002")]` with
  `[Replaces]`.
- `[Replaces]` makes the class the game's object itself: every unit of that type is made as the class.
- `Unit.Create<Champion>(player, x, y)` makes one in the game, facing 270 degrees unless a facing follows.

## Components and reactions

A reaction is a method on the thing involved: `OnDamaged`, `OnDeath` and `OnKill` on a unit, `OnCast` on an ability,
`OnPickedUp` on an item. A component is a class that carries reactions and state, and is added to a unit, item,
destructable, player or the game: it inherits `UnitComponent`, `ItemComponent`, `DestructableComponent`,
`PlayerComponent` or `GameComponent`.

```csharp
public sealed class KillCounter : UnitComponent
{
    private int kills;

    public override void OnKill(Unit victim)
    {
        kills++;
        Unit.Player.Print("Kills: " + kills);
    }
}
```

- A type lists the components its instances start with in its constructor. `unit.Add(new KillCounter())` adds one in the
  game, and `End()` removes it.
- Reactions to the whole map (`OnGameStart`, `OnUnitDied`, `OnUnitCast`, `OnLeave`) go on a game or player component.
- An event runs the type's reaction first, then its components' in the order added, then the game's and the
  players'. A reaction that fails is reported, and the others still run.
- A buff is a class that inherits `Buff`, or a standard buff to show its icon. `Duration` and `Tick` set its time,
  `OnApply`, `OnTick` and `OnExpire` are its reactions, and `unit.Apply(new Burn())` puts it on a unit.
- `Timer.After(seconds, action)` and `Timer.Every(seconds, action)` run code later. A timer, buff or component
  belongs to whatever made it, and ends with it.
- Game work starts in a reaction. Top-level code and static fields make nothing in the game.

## Damage

`OnDamaged` sees a hit after armor. For more, add the `Moonwell.Damage` package to the project, at the same version as
Moonwell; the build starts it, and a map without it carries none of its code. Its reactions come through interfaces,
which a unit's class, its components and buffs implement, writing only the methods they need as public methods:

```csharp
public sealed class Shield : Buff, IDamageTarget
{
    public void OnTaken(Hit hit) { hit.Amount *= 0.5f; }
}
```

| Stage | When | `IDamageSource` | `IDamageTarget` | `IDamageWatch` |
|---|---|---|---|---|
| Before armor | the hit starts | `OnDealing` | `OnTaking` | `OnBeforeArmor` |
| After armor | armor has counted | `OnDealt` | `OnTaken` | `OnAfterArmor` |
| Lethal | the hit would kill | `OnKilling` | `OnLethal` | `OnLethal` |
| Landed | 0 s later, the life has dropped | `OnLanded` | `OnHurt` | `OnLanded` |
| Burst | 0 s later, once per source | `OnBurst` | | `OnBurst` |

- Each stage runs the source and its components, then the target and its components, then the game's and the
  players' components that implement `IDamageWatch`. `OnDamaged` and `OnDealtDamage` still run, first in the
  after-armor stage.
- A `Hit` has the amount, `Original`, `IsAttack`, `IsSpell`, `IsMelee`, `IsRanged`, `IsCode` and `Tag`. Before armor
  a reaction may also change `DamageKind`, `WeaponSound` and the target's `Armor` for this hit. In the lethal stage
  `hit.Spare()` keeps the target alive; `hit.Killed` tells in the landed stage.
- `source.Damage(target, 10f, DamageKind.Fire, tag: MyTag.Firebolt)` deals damage from code and returns the `Hit`.
- Damage dealt from a reaction belongs to the hit it reacted to; a chain of 8 such hits ends with an error, so two
  units that hit each other back never loop.
- A hit spell immunity stops runs before armor only.

## Players

`Player.One` to `Player.TwentyFour` are the player slots, as the lobby numbers them; `Player.Get(slot)` takes the
slot from 0, so `Player.Get(0)` is `Player.One`. A slot is not a colour: a map can recolour its players.
`Player.NeutralAggressive`, `NeutralPassive`, `NeutralVictim` and `NeutralExtra` are the neutral players, and
`StartX` and `StartY` a player's start location.

`player.Print("Kills: 3")` shows a line to one player and `Game.Print("Wave 2")` to everyone; both take a time on
screen, `seconds: 10f`. `Gold`, `Lumber`, `FoodUsed`, `FoodCap`, `FoodCapCeiling`, `GoldGathered`, `LumberGathered`,
`GoldUpkeepRate` and `LumberUpkeepRate` are the player's own: `player.Gold += 25`.

`Player.Playing` is every player in the game, people and computers, and `Player.Users` the people:
`foreach (var player in Player.Users)`. `player.SlotState` is `Empty`, `Playing` or `Left`, and `player.Controller`
says who plays it. `OnLeave(Player player)` runs on the game's components, then the leaving player's.

`Name`, `Color` and `Team` are read and set, and `Race` read; setting `Color` also recolours the player's units.
`a.SetAlliance(b, Alliance.Allied | Alliance.SharedVision)` sets what `a` gives `b`, one way as the game keeps it, and
`a.AllianceWith(b)`, `a.IsAlly(b)` and `a.IsEnemy(b)` read it. `LevelOf<T>()` and `SetLevel<T>(2)` take an upgrade class;
`SetMaxAllowed<T>(1)` a unit or upgrade class, where 0 forbids it and `int.MaxValue` is no limit; and
`SetAvailable<T>(false)` an ability class.

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

The build also rejects what CSharp.lua cannot compile: collection expressions, `[1, 2]` (write `new List<int> { 1, 2 }`),
and `using static Moonwell.Natives` (write `Natives.X`).

An error in a reaction shows on the screen with its C# file and line, and is written in full to
`Documents\Warcraft III\CustomMapData\Moonwell\log.txt`. A release build leaves the message off the screen and keeps
the file and line.

To place objects declared in code in World Editor, save `build\MyMap.w3x` from World Editor as the `map/` folder. The
next build keeps the placed units; code wins over any object it also declares.
