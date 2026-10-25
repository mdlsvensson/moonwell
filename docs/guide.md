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

The game opens the map and shows the Moonwell version. Player 1 has a Champion, a Paladin of the map's own, with
three footmen to fight. Learn Smite and cast it: the screen names the target. Each kill shows the Champion's count and heals it a tenth of its life.

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
        MaxLife = MaxLife + 200;
        HeroAbilities.Replace<Human.Abilities.HolyLight, Smite>();
        Components.Add(new KillCounter());
    }
}
```

- A field with levels takes `Levels(75f, 150f, 225f)`, `PerLevel(level => ...)` to compute each level, or one value
  for every level: an upgrade's `Name = "Sharpened Blades"`.
- A field World Editor offers a list of values for holds an enum of them, named as the game's data spells them:
  `AttackType = AttackType.Magic`. One that takes several, such as the targets, holds flags:
  `Attack1TargetsAllowed = Targets.Air | Targets.Ground`. A flags field of 0 has none of them; `Targets.None` is
  World Editor's target "none", a value of its own. The numbers are Moonwell's, in World Editor's order, not the
  natives' (JASS's `ATTACK_TYPE_MAGIC` is 4, `AttackType.Magic` is 6): never cast one to or from a native's value.
  In the game an enum of `Moonwell.Objects` has its numbers only, no names: `ToString` (also `choice?.ToString()`), a
  string made with one (`+`, `+=`, interpolation, or a call that takes an `object`: `string.Format`,
  `StringBuilder.Append`, a method of your own) and `Enum.GetValues` are rejected as error MW0010, a nullable choice
  too; compare with its members, `|`, `&` and `HasFlag`.
- `AttackType` and `DefenseType` are core Moonwell's (`Moonwell.AttackType`), shared with the damage package, and keep
  their names in the game. A file writes them with `using Moonwell;`; a name of the old `Moonwell.Objects.AttackType`
  is error MW0011.
- A field that a `Unit`, `Hero` or `Item` also has as a live value has one name for both. In the constructor it sets
  what every object of the class starts with; anywhere else it is this object (`MaxLife += 10` in `OnKill`). The
  names are `MaxLife`, `MaxMana`, `Mana` (what it starts with), `Scale`, `FlyHeight` and `Name` on a unit and hero,
  `Level` on a unit that is not a hero, `Strength`, `Agility` and `Intelligence` on a hero, and `Charges` on an item.
  They were `HitPoints`, `ManaPoints`, `ManaInitialAmount`, `ScalingValue`, `Height`, `UnitLevel`,
  `StartingStrength`, `StartingAgility`, `StartingIntelligence` and `NumberOfCharges`, which are gone. Outside the
  constructor the old names read the declared value; write `Default.MaxLife` (`Default.Name`) for that. An item's `Level`,
  `Scale` and `HitPoints`, a doodad's `Scale` and a destructable's `HitPoints` keep their names, and six ability fields are
  named apart: `Inferno.InfernalDuration`, `Taunt.PreferHostiles` and `PreferFriendlies`, `WindWalk.BackstabDamage` and
  `Backstab`, `HealingModifier.HealingPercent`.
- A field with no live value (`Model`, `Tooltip`, `GoldCost`) keeps World Editor's name, is set only in the constructor,
  and reads the declared value anywhere.
- A hero's `MaxLife`, `MaxMana` and `Mana` in its constructor are the totals it starts with: `MaxLife = 800` starts a
  Paladin at 800 life. The build takes off what its starting strength and intelligence give (25 life per strength and
  15 mana per intelligence, or the map's own in `war3mapMisc.txt`) and writes the rest as the base field. A total the
  attributes alone pass fails the build with its line: `Champion.MaxLife is 400; its 30 strength already give 750
  (751 at least)`. A hero class that sets `Strength` but not `MaxLife` keeps its base, so its life follows the strength.
  A hero starts at level 1: `Level = ...` or `Set("ulev", ...)` in a hero's constructor is error MW0012; use `SetLevel`
  in the game. A unit's `Level` is set only in its declared class's constructor, since the game has no setter: anywhere
  else it is error MW0015, and a build error too.
- A list takes `Set<A, B, C>()` (the whole list, in order), `Replace<Old, New>()` (in the old one's place), `Add<T>()`
  and `Remove<T>()`.
- The tooltips are World Editor's four: `Tooltip` (Tooltip - Normal), `TooltipExtended` (Tooltip - Normal - Extended),
  `TooltipLearn` (Tooltip - Learn) and `TooltipLearnExtended` (Tooltip - Learn - Extended).
- A field with no name of its own is set by its id: `Set("Htb1", 2, 250f)`.
- A constructor may compute, but must not call the game: the build fails at such a line. A live member with no declared
  field there (`Life = 300`, `Kill()`, `X`) is error MW0012, which says what to set instead where there is one: a unit
  starts with full life, so set `MaxLife`.
- A class with no `[ObjectId]` is error MW0013, and the build names a free id to use. A buff also names its carrier,
  the hidden ability that shows its icon: `[ObjectId("B001", Carrier = "A002")]`, or `[ObjectId(Carrier = "A002")]`
  with `[Replaces]`. An id is four characters, and only a hero's starts with an upper-case letter (MW0014).
- `[Replaces]` makes the class the game's object itself: every unit of that type is made as the class.
- `Unit.Create<Champion>(player, x, y)` makes one in the game, facing 270 degrees unless a facing follows;
  `Item.Create<EmberBlade>(x, y)` makes an item.

Each value a class sets is checked against World Editor's limits. One outside them fails the build with its line, and
a constant is marked in the editor as error MW0009:

```
Heroes/Giant.cs:12: Giant.MaxLife is 600000; Hit Points Maximum (Base) (uhpm) allows 1 to 500000; [BeyondLimits] goes past it
```

`[BeyondLimits(nameof(MaxLife))]` on the class goes past a range on purpose. It lifts ranges, lengths and counts
only: an empty name, a negative on a field the game never lets go below 0, and a choice that is none of the enum's
values still fail. The values a standard class has are the game's, and are never checked.

To see a standard object's values, go to its class: Go to Definition on `Paladin` opens the class's source, which
`Moonwell.Objects` embeds in its library, in an editor that navigates to embedded sources (the editors on Microsoft's C#
language server do). Its constructor sets each value of the base tables, World Editor's
Default. A map on another data set plays that set's values for the fields it leaves unset.

### The default object and the name

`Default` is the class's declared values as an object, which never changes when the unit does. Every unit, hero and
item has it, a plain standard unit too: `unit.Default.MaxMana` is 300 whatever this Champion's is now. From the class
alone it is `Unit.Default<Champion>().MaxMana`: `Unit.Default<T>()` takes a unit class (`Hero.Default<T>()` works too),
`Item.Default<T>()` an item class. On a declared class `Default` is typed as its kind base (`StandardUnit`,
`StandardHero` or `StandardItem`), so it holds every field of it (`Default.Model`, `Default.GoldCost`); the class's own
members come through `Unit.Default<T>()`. Its live actions do nothing. In the class's own constructor the default
object is what the constructor is making, so `Default` and `Unit.Default<T>()` of the class there are error MW0012:
read the field itself (`MaxLife`, not `Default.MaxLife`).

`Default` of a type made or changed in World Editor reads its values as World Editor made them; a class of the map with
its id wins over them.

`unit.Name` reads the game's name, which is in each player's language, so reading it is error MW0002 outside
`Local.For`; setting it is fine anywhere outside `Local.For`. Reading `Name` in a declared class's constructor, and on
`Default`, is allowed, and the same on every machine. A local copy of a default (`var d = unit.Default; d.Name`) is
still MW0002, a false alarm: write `unit.Default.Name`. A hero's `Name` is its type's ("Paladin"). `Hero.ProperName`
("Arius the Seeker") has no declared value: it is read inside `Local.For` only, set anywhere outside it, and empty on a
default.

## Components and reactions

A reaction is a method on the thing involved: `OnDamaged`, `OnDeath` and `OnKill` on a unit, `OnCast` on an ability,
`OnPickedUp` on an item. A component is a class that carries reactions and state, and is added to a unit, item,
destructable, player or the game: it inherits `UnitComponent`, `ItemComponent`, `DestructableComponent`,
`PlayerComponent` or `GameComponent`.

```csharp
public sealed class KillCounter : UnitComponent
{
    private int kills;

    public override void OnKill(Unit killedUnit)
    {
        kills++;
        Unit.Player.Print("Kills: " + kills);
    }
}
```

- A type lists the components its instances start with in its constructor. `unit.Add(new KillCounter())` adds one in the
  game, and `unit.Remove<KillCounter>()` removes it (`End()` ends a component from inside). `HasEnded` is false inside
  `OnRemoved`, `OnLeave` and `OnEnd`, and true once the removal or `End()` returns.
- Reactions to the whole map (`OnGameStart`, `OnUnitCreated`, `OnUnitDied`, `OnUnitCast`, `OnLeave`) go on a game or
  player component. `OnUnitCreated` runs once for each unit, when it comes into the game; a unit's own is `OnCreated`.
- A reaction's parameters say the role: `OnPickedUp(Unit carrierUnit)`, `OnUnitDied(Unit dyingUnit, Unit? killerUnit)`,
  `OnKill(Unit killedUnit)`. The killer is null when no unit's damage killed: `Kill()` or `Life = 0` from code, timed
  life running out, or a unit's own damage.
- An event runs the type's reaction first, then its components' in the order added, then the game's and the
  players'. A reaction that fails is reported, and the others still run.
- A buff is a class that inherits `Buff`, or a standard buff to show its icon. `Duration` and `Tick` set its time,
  `OnApply`, `OnTick` and `OnExpire` are its reactions, and `unit.Apply(new Burn())` puts it on a unit.
- `Timer.After(seconds, action)` and `Timer.Every(seconds, action)` run code later. A timer, buff or component
  belongs to whatever made it, and ends with it.
- Game work starts in a reaction. Top-level code and static fields make nothing in the game.

## Units, heroes and items

A unit's members read and change the game's. Once the game has removed a unit or item they give the last value read or
set (0 when never read, as in `OnLeave` after the game removed it), and setting does nothing. In the quarter second
before Moonwell notices a removal, a read that gives 0 on a removed unit keeps the last value too. A read is safe in
`Local.For`, a change is not.

| Unit | |
|---|---|
| `Life`, `MaxLife`, `Mana`, `MaxMana` | read and set; a `Life` under 0.405 kills, above the most it clamps; the most are whole numbers |
| `X`, `Y` | read; `MoveTo(x, y)` through the game's pathing, `PlaceAt(x, y)` with none, not even the map's edge |
| `Facing` | read and set (the unit turns, so a read at once gives the old one); `FaceNow(degrees)` turns at once, a walking unit turns back |
| `Level` | read; a hero's hero level; a unit's level is set only in its declared class's constructor |
| `IsAlive`, `Kill()` | whether it lives; kills it, with `OnDeath` run |
| `IsPaused`, `Scale`, `FlyHeight` | read and set; a paused unit takes no orders but still animates; `Scale` reads its type's scale until set; `FlyHeight` also lifts a ground unit |
| `Name` | read inside `Local.For` only (each player's language), set anywhere outside it |
| `Tint(red, green, blue, alpha)` | colours the model, each value clamped to 0-255 |
| `Order(order)`, `OrderAt(order, x, y)`, `OrderOn(order, target)`, `Stop()` | orders by the game's order string (`"move"`, `"attack"`) |
| `InventorySize`, `ItemIn(slot)`, `Give(item)`, `Drop(item)` | the inventory; `Give` and `Drop` run the pick-up and drop reactions inside the call; `Give` answers false, with no reaction, for a full inventory; giving an item the unit holds runs a drop and then a pick-up |

| Hero (a unit the game makes as a hero) | |
|---|---|
| `Experience` | read and set; the levels that come with it are given |
| `SetLevel(level)`, `Revive(x, y)` | `showEffect:` turns the effect off |
| `Strength`, `Agility`, `Intelligence` | the base, read and set |
| `StrengthWithBonus`, `AgilityWithBonus`, `IntelligenceWithBonus` | with items and auras, read |
| `SkillPoints` | read and set |
| `ProperName` | read inside `Local.For` only, set anywhere outside it |

| Item | |
|---|---|
| `Charges`, `Player` | read and set; 0 charges leaves the item in place; `Player` is the owner, which picking up or dropping does not change |
| `X`, `Y` | read; `MoveTo(x, y)` goes to a pathable spot; while a unit carries the item they are where it was picked up, so read the carrier's place |
| `Carrier`, `IsCarried` | the unit that picked it up, while it still holds it: null for an item in a bag or equipped (the 3.0 inventory), and in `OnDropped` it is still the unit, except for a drop after `RemoveUnit` (below), where it is already null |

`ItemIn` is not for `Local.For`: it takes in an item Moonwell has not seen, which has to happen on every machine.
Moonwell listens for pick-ups and drops from the start, so an item a unit picks up is taken in; one a unit held before
the map began has no `Carrier`. A unit that is not a hero drops its items when it dies, with no drop reactions; a hero
keeps them. When a unit is removed from the game (`RemoveUnit`), its items leave it inside the call, but the game fires
the drops after it: `Carrier` is already null then, and the drop reactions run for an object about to be swept. The
items of a removed hero are removed with it, as seen; other units are assumed to do the same. A member's exact
behaviour is in its comment.

## Damage

`OnDamaged` sees a hit after armor. `source.Damage(target, 10f)` deals damage from code, as a spell does: magic,
of the spells attack type. `DamageKind.Fire`, `attackType:`, `isAttack:`, `ranged:`, `sound:` and `tag:` (read back as
`damage.Tag`) change it. The reactions run inside the call, which returns the damage they saw, or null when the game
dealt nothing.

For more, add the `Moonwell.Damage` package to the project, at the same version as
Moonwell (the template's `.csproj` has the line, commented). The build starts it once the map's code uses one of its
types, and a map that does not carries none of its code. Its reactions come through interfaces, which a unit's class,
its components and buffs implement, writing only the methods they need as public methods:

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
  a reaction may also change `DamageKind`, `WeaponSound`, `AttackType`, the target's `DefenseType` and its `Armor` for
  this hit; the target has its own defense type and armor back afterwards. In the lethal stage
  `hit.Spare()` keeps the target alive; `hit.Killed` tells in the landed stage.
- `source.Damage(...)` goes through the stages too, and what it returns is the `Hit`: `(Hit?)source.Damage(target, 10f)`.
- Damage dealt from a reaction belongs to the hit it reacted to; a chain of 8 such hits ends with an error, so two
  units that hit each other back never loop.
- A hit spell immunity stops runs before armor only.

## Maths

Add the `Moonwell.Maths` package (the template's `.csproj` has the line, commented) for vectors, angles, easing,
bounds, polygons and raycasts. A map that does not reference it carries none of its code.

```csharp
var start = target.Position();
var end = start.Offset(caster.AngleTo(target), 300f);
target.PlaceAt(Vector2.Lerp(start, end, Ease.OutQuad(t)));
```

- Angles are degrees, as `Facing`: 0 along +X, 90 along +Y. `MathF.Sin` takes radians; use `Angles.Sin`.
- `Vector2`, `Vector3`, `Bounds`, `Ray` and `RayHit` are values that never change once made, so they are shared and
  never copied. Their names follow Godot's; `Vector2.Up` is north on the map, and `AngleTo` is the facing to a point.
- In the game a vector is a table to make: in code that runs every tick on many objects, use the float forms in
  `Maths` (`Maths.Distance`, `Maths.AngleBetween`, `Maths.Polar`), which make none.
- `unit.Position()` is a method. `PlaceAt` ignores pathing and keeps the unit's order, which a push wants; `MoveTo`
  goes through pathing and ends it.
- `Raycast.Circle`, `Segment`, `Bounds` and `Polygon` give the first hit along a segment in an `out` value, with a
  normal for `Bounce` and `Slide`.

## Interactions and finding

Add the `Moonwell.Interactions` package (the template's `.csproj` has the line, commented) for things that react to what
comes near them, gizmos, and finding units, items and destructables. A map that does not reference it carries none of
its code.

```csharp
public sealed class FireAura : Interaction<Unit, Unit> // held by a unit, pairs with units
{
    public FireAura() { Range = 300f; Tick = 0.5f; }
    public override bool Accepts(Unit other) => Holder.Player.IsEnemy(other.Player);
    public override void OnTick(Pair pair) { pair.Other.Life -= 10f; }
}

unit.Interact(new FireAura()); // made inside a buff, it ends with the buff

public sealed class Well : Gizmo // a map's own object with a place
{
    protected override void OnCreated() { this.Interact(new Healing()); Timer.After(20f, End); }
}

var well = Gizmo.Create(new Well(), x, y, caster.Player);
var near = Find.InRange<Unit>(caster.Position(), 500f); // a new list
Find.InRange(x, y, 500f, targets);                      // fills a list the map keeps
```

- `Range` is centre to centre and flat. A pair is found at the next tick after they come within range, so a 1 s tick
  may start up to 1 s late; `Tick = 0.03125f` for something fast. `OnPart` runs once, when the pair ends.
- What has a place: a unit in the game, alive, not hidden and not loaded; an item on the ground; a live destructable; a
  gizmo until it ends. Players and abilities have none.
- `Find` has `InRange`, `InRect`, `InPolygon`, `AlongLine` and `Closest`, each in two forms: one gives a new list, the
  other fills a list the map keeps, which makes none. Use that one in code that runs every tick.
- `Interactions.CellSize` (128), `CellCheck` (0.1), `Slack` (64) and `Step` (1/32) are set in the map's `Map` constructor.
- Everything runs on every machine, so `Find` is fine inside `Local.For`: there it reads and changes nothing, and for
  a kind nothing outside has asked for yet it looks through the objects Moonwell knows, which is slower.
- `IsTree()` is a method and needs `Moonwell.Objects` in the map; the build warns when it is missing.

## Players

`Player.One` to `Player.TwentyFour` are the player slots, as the lobby numbers them; `Player.Get(slot)` takes the
slot from 0, so `Player.Get(0)` is `Player.One`. A slot is not a colour: a map can recolour its players.
`Player.NeutralAggressive`, `NeutralPassive`, `NeutralVictim` and `NeutralExtra` are the neutral players, and
`StartX` and `StartY` a player's start location.

`player.Print("Kills: 3")` shows a line to one player and `Game.Print("Wave 2")` to everyone; both take a time on
screen, `seconds: 10f`. `Package.Version` is the version of Moonwell the map is built with. `Gold`, `Lumber`, `FoodUsed`, `FoodCap`, `FoodCapCeiling`, `GoldGathered`, `LumberGathered`,
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

## C# the game's Lua cannot run

Moonwell turns your C# into Lua with CSharp.lua, and the game runs the Lua. A few C# forms stop the compiler, write
Lua that does something else, or call a library function the Lua core does not have. The analyzer rejects them as
errors with their line, so the map does not build and then fail on the screen. Nothing silences them (`[SyncUnchecked]`
does not), because the Lua cannot run them anywhere. The list comes from running about 770 small programs on .NET and
in Lua and comparing what they printed.

| Rule | Rejects | Write instead |
|---|---|---|
| MW0005 | `decimal` (with `long`, `ulong`, `uint`, `double`) | `int` or `float` |
| MW0017 | in a `switch` expression: `> 10 =>`, `int =>`, `var n`, `not 1`; in any pattern: `(...)`, a name inside `{ X: int y }`, `not null` inside `{ }`, `.. var rest` | a `when` on a named variable (`int n when n > 10`), or an `if` |
| MW0018 | `x is (1, _)`, `p is Pt(var a, var b)`, `p is { Inner.X: 9 }` | `{ X: 1 }`, a `switch`, `{ Inner: { X: 9 } }` |
| MW0019 | `-N` with `const int N = -5` (Lua reads `--5` as a comment) | write the number, `0 - N` |
| MW0020 | `var c = arr[0]; c.X = 6;` on a struct (it changes `arr[0]`) | make a new struct, or write `c` back |
| MW0021 | a primary constructor's parameter in a field initializer or in `: Base(n)` (it is 0) | assign it in a constructor body |
| MW0022 | `var (a, b) = (1, 2)`, `(x[0], x[1]) = (x[1], x[0])`, a tuple turned into text | `(int a, int b) = (1, 2)`, a temporary, its items |
| MW0023 | a user-defined `++`, `--`, `operator true/false`; `static abstract`; a record deriving from a record; one class implementing `IG<int>` and `IG<string>`; `with` on an anonymous type | a method; an ordinary class |
| MW0024 | `a[1..3]`, `s[1..]`, `list[^1]`, `Index`, `Range` (`array[^1]` is fine) | `Substring`, `Skip`/`Take`, `list[list.Count - 1]` |
| MW0025 | a `ref` return, `ref int r = ref First(a)`, `ref int r = ref a[i]` | index the array each time |
| MW0026 | `params List<int>`, `params` in a lambda | `params int[]` |
| MW0027 | `s.Split("--")` (runs out of memory), `StartsWith('a')`, `EndsWith('a')`, ignore-case `IndexOf`/`StartsWith`/`EndsWith`, `"abc".Reverse()` | split on a char, a string argument, lower-case both |
| MW0028 | a `char` or an enum given to `string.Format`, `Console.WriteLine(object)`, `StringBuilder.Append(object)`, `string.Join` (shown as its number) | `.ToString()` |
| MW0029 | `catch (NullReferenceException)`, `catch (IndexOutOfRangeException)`, a pattern variable in a `catch ... when` | `catch (Exception)`, test first |
| MW0030 | a library member the Lua core lacks: `StringComparer`, `Guid`, `Task`/`await`, `Range`, LINQ `Append`/`Prepend`/`Zip`/`TakeLast`/`ToHashSet`/`Chunk`, `List.ConvertAll`, `StringBuilder.Replace`/`Insert`, `Dictionary.GetValueOrDefault`, `TryPeek`/`TryPop`, `Environment.NewLine`, and more (the rule's description lists them); also `HashSet.SetEquals`, `Math.DivRem`, `d.Remove(k, out v)` and `ToList()` of a `Dictionary`, which give wrong answers | `TryGetValue`, `"
"`, a loop |
| MW0031 | `checked(a + b)`, `checked { }` (an overflow is never detected) | test the range yourself |
| MW0032 | `Enum.Parse`, `Enum.GetValues`/`GetNames`, `ToString("D")` on an enum, a `[Flags]` combination turned into text, `(Color)5 + "x"` | a `switch` on the value, `HasFlag` |
| MW0033 | `new A { ... }.M()` or `.X` twice in one statement, or once among three initializers (they share a Lua temporary) | put each in a local |
| MW0034 | two `using var` in one block (disposed in the wrong order) | nested `using ( ) { }` |

What works is most of the language: classes, structs, records, interfaces and default methods, generics with
constraints, enums, delegates, events, `switch` statements, `is` patterns with constants, types and properties,
`??`, `?.`, collection expressions, `foreach` over every collection, `yield`, lambdas and closures, `ref`/`out`/`in`
parameters, `lock`, string interpolation, `List`, `Dictionary`, `HashSet`, `Queue`, `Stack`, arrays and most of LINQ.
`float` and `int` arithmetic is the game's, see MW0005. Two things the analyzer cannot see: an iterator with a
`try/finally` that the caller leaves early with `break` skips the `finally`, and `byte`/`short` arithmetic does not wrap.

## The build

`moonwell build` builds the map in the current folder into `build\MyMap.w3x`; `moonwell run` builds it and starts the
game with it. Both make a development build: it checks the values Moonwell receives and keeps the Lua readable.
`--release` makes the release build. `moonwell run --game <exe>` names the game when it is not in its usual place.

The build also rejects `using static Moonwell.Natives` (write `Natives.X`), which hides `Player` behind
`Natives.Player`. The build does not catch a
`Dictionary` keyed by a map's own class that has a static field with a value: the map can stop while it loads, with
nothing on the screen. Key by the handle, or set the field on first use. Moonwell's classes, `Unit` and the rest, are
safe keys.

An error in a reaction shows on the screen with its C# file and line, and is written in full to
`Documents\Warcraft III\CustomMapData\Moonwell\log.txt`. A release build leaves the message off the screen and keeps
the file and line.

To place objects declared in code in World Editor, save `build\MyMap.w3x` from World Editor as the `map/` folder. The
next build keeps the placed units; code wins over any object it also declares.
