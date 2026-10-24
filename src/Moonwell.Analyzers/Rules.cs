using System.Collections.Immutable;
using Microsoft.CodeAnalysis;

namespace Moonwell.Analyzers;

/// <summary>
/// The multiplayer mistakes the analyzer rejects (decision 13); <c>[SyncUnchecked]</c> on a method turns all of them off there.
/// Then the C# CSharp.lua cannot compile, which nothing turns off.
/// </summary>
public static class Rules
{
    private const string Category = "Multiplayer";

    private const string Silence = " [SyncUnchecked] on the method turns this check off.";

    public static readonly DiagnosticDescriptor StateInLocal = new(
        "MW0001",
        "Changes the game inside Local.For",
        "{0}; inside Local.For it runs on one machine only, which desyncs the game",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "Code inside Local.For runs on one player's machine. It may change only what that player sees or hears; creating handles or changing the game there desyncs the other machines." + Silence);

    public static readonly DiagnosticDescriptor LocalOutside = new(
        "MW0002",
        "Machine-local value outside Local.For",
        "'{0}' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method{1}",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "A machine-local value (the local player, the camera, the mouse, frame text, localised names) differs between machines, so game logic that uses it desyncs." + Silence);

    public static readonly DiagnosticDescriptor HandleId = new(
        "MW0003",
        "GetHandleId",
        "Handle ids differ between machines; key by the handle itself, or by a number the map assigns",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "Each machine frees handle ids on its own, so the same object can have different ids." + Silence);

    public static readonly DiagnosticDescriptor HashLoop = new(
        "MW0004",
        "Loop over a Dictionary or HashSet",
        "A {0} loops in no fixed order, which can differ between machines; use OrderedMap or OrderedSet",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "In the game's Lua a Dictionary or HashSet gives its items in no fixed order. Lookups are fine; loops are not." + Silence);

    public static readonly DiagnosticDescriptor NumberType = new(
        "MW0005",
        "Number type the game cannot carry",
        "'{0}' {1}",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "The game's Lua has 32-bit integers and single-precision floats." + Silence);

    public static readonly DiagnosticDescriptor Desyncs = new(
        "MW0006",
        "Desyncs wherever it runs",
        "'{0}' changes the game inside its own GetLocalPlayer() block, so it desyncs wherever it runs",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "A blizzard.j function that compares GetLocalPlayer() and then changes the game, such as SelectGroupForPlayerBJ, desyncs on every call. Do its local part inside Local.For with what the analyzer allows there." + Silence);

    public static ImmutableArray<DiagnosticDescriptor> All { get; } =
        ImmutableArray.Create(StateInLocal, LocalOutside, HandleId, HashLoop, NumberType, Desyncs);

    private const string LuaCategory = "Lua";

    public static readonly DiagnosticDescriptor StaticNatives = new(
        "MW0008",
        "using static Moonwell.Natives",
        "using static Moonwell.Natives hides Moonwell.Player behind Natives.Player; write Natives.X",
        LuaCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "Player.Get(0) fails with CS0119 because Natives.Player(int), imported by using static, wins over the type Moonwell.Player.");

    public static readonly DiagnosticDescriptor ChoiceNames = new(
        "MW0010",
        "Name of a Moonwell.Objects enum",
        "'{0}' needs the names of {1}, which the game's Lua does not have: there a choice is only its number (ArmorType.Metal shows as 1) and Enum.GetValues stops; compare with its members, or show (int) of it",
        LuaCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "The enums of Moonwell.Objects (ArmorType, Targets, ...) are ignored by CSharp.lua: the game's Lua has their numbers only. ToString (also x?.ToString()), a string joined (+ or +=) or interpolated with one, one given to an object parameter (string.Format, string.Concat, StringBuilder.Append; Equals is left alone), and Enum methods given one (GetValues, GetNames, GetName, IsDefined, Parse, TryParse, Format, ToObject) give the number where .NET gives the name, or stop. Comparing, |, & and HasFlag work.");

    public static readonly DiagnosticDescriptor NullableBranch = new(
        "MW0016",
        "Conditional whose first branch can be null or false",
        "'{0}' gives its second branch in the game when its first is null or false, since CSharp.lua writes it with and/or; write an if",
        LuaCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "CSharp.lua writes c ? a : b as c and a or b when a is a value: Lua's or then gives b when a is nil or false. So a first branch that is a nullable value (int?, float?, bool?, an enum's or a struct's), or a type parameter's value that can be a bool, gives the second branch when it is null (or false), with no error. A first branch that is a class, a plain bool or null is written with if and is right; so is a plain value, one made nullable from a plain value other than a bool ((int?)i), one the condition has checked for null (n != null ? n : x), and c ? n : null, which gives nil either way, except for a bool?.");

    /// <summary>The rules of C# that CSharp.lua cannot compile.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Lua { get; } = ImmutableArray.Create(StaticNatives, ChoiceNames, NullableBranch);

    private const string ObjectsCategory = "Objects";

    public static readonly DiagnosticDescriptor FieldRule = new(
        "MW0009",
        "Value World Editor does not allow",
        "{0}",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "A constant a class puts on a field of a standard object (an assignment, an argument of Levels, a Set by field id) is outside what World Editor allows: a number outside its range, a negative where none is allowed, an empty required name, a text or list that is too long, or a choice that is no value of its enum. The message is the one the build gives. [BeyondLimits(nameof(Property))] on the class goes past a range or a length; it never allows a negative, an empty name or a wrong choice. It also reports a [BeyondLimits] name that is no property, which lifts nothing.");

    /// <summary>The rules for the values a map's classes put on object fields.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Objects { get; } = ImmutableArray.Create(FieldRule);

    public static readonly DiagnosticDescriptor MovedChoice = new(
        "MW0011",
        "Choice enum moved to namespace Moonwell",
        "{0} is Moonwell.{1} since Moonwell 0.2.1, which the damage package uses too: write Moonwell.{1}, or {1} with using Moonwell",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "AttackType and DefenseType left Moonwell.Objects for core Moonwell in 0.2.1, so that object data and the damage package share one of each. The compiler's own error does not name the new enum: CS0234 for Moonwell.Objects.AttackType, CS0103 for AttackType without using Moonwell, and CS0176 for AttackType.Magic in a standard class without using Moonwell, where AttackType is the property.");

    /// <summary>The rules for names that moved between Moonwell's namespaces.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Moved { get; } = ImmutableArray.Create(MovedChoice);

    public static readonly DiagnosticDescriptor LiveInDeclaration = new(
        "MW0012",
        "The game in a declared class's constructor",
        "'{0}' {1}",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "The build runs the constructor of a class that inherits a standard unit, hero or item to read its fields, before there is a game. A live value or action of Unit, Hero or Item there (Life, X, Kill, Add) fails the build with only an exception's message. The live values a declared field shares (MaxLife, MaxMana, Mana, Level, Scale, FlyHeight, Name; a hero's Strength, Agility and Intelligence; an item's Charges) are the class's declared values there, except a hero's Level (and Set(\"ulev\", ...)): a hero starts at level 1, so its level is not declared. A unit starts with full life, so Life has no declared value: set MaxLife. A component goes to Components.Add(...). The class's own default object (Default, or Unit.Default<T>() of the class) is what the constructor is making: read the field itself. Code in a lambda or a local function is left alone, since it may run later in the game.");

    public static readonly DiagnosticDescriptor LevelOutsideConstructor = new(
        "MW0015",
        "A unit's level set outside its declared class's constructor",
        "a unit's level is set only in its declared class's constructor",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "The game has no setter for a unit's level, so Level is a unit's declared value: it is set in the constructor of the class that declares the unit (and in a lambda or local function there), and only read anywhere else. A hero's level is set with SetLevel; a hero's constructor does not declare it. The build rejects the same set.");

    /// <summary>The rules for the constructor of a declared object class.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Declared { get; } = ImmutableArray.Create(LiveInDeclaration, LevelOutsideConstructor);

    public static readonly DiagnosticDescriptor NoId = new(
        "MW0013",
        "A declared object without its id",
        "{0} has {1}; moonwell build names a free one",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "Every class that declares an object of the map's own needs its id, as World Editor shows it, in [ObjectId(\"h001\")]; a [Replaces] class has its standard object's. A buff also names its carrier, the hidden ability that shows its icon: [ObjectId(\"B001\", Carrier = \"A001\")]. The build fails without them and names a free id to use.");

    public static readonly DiagnosticDescriptor BadId = new(
        "MW0014",
        "An object id the game cannot take",
        "{0} {1}",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "An object id is four characters. The game makes a unit whose id starts with an upper-case letter a hero, and only such a unit: a hero's id starts with one, another unit's does not.");

    /// <summary>The rules for a declared object's id.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Ids { get; } = ImmutableArray.Create(NoId, BadId);
}
