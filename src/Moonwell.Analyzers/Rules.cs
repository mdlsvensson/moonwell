using System.Collections.Immutable;
using Microsoft.CodeAnalysis;

namespace Moonwell.Analyzers;

/// <summary>
/// The multiplayer mistakes the analyzer rejects (decision 13); <c>[SyncUnchecked]</c> on a method turns them off in its
/// own body, except <c>decimal</c>, and Local.For still follows the method. Then the C# CSharp.lua cannot compile, which
/// nothing turns off.
/// </summary>
public static class Rules
{
    private const string Category = "Multiplayer";

    private const string Silence = " [SyncUnchecked] on the method (or the lambda or local function) turns this check off in its own body; Local.For still follows a method so marked.";

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
        description: "A machine-local value (the local player, the camera, the mouse, frame text, localised names) differs between machines, so game logic that uses it desyncs. So do .NET's clock (DateTime.Now, UtcNow and Today, Environment.TickCount, a Stopwatch's reading) and an object's address in the game's Lua: GetHashCode of a handle or of a class without its own, RuntimeHelpers.GetHashCode, and a handle turned into text." + Silence);

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
        "{0} loops in no fixed order, which can differ between machines; use OrderedMap or OrderedSet",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "In the game's Lua a Dictionary or HashSet gives its items in no fixed order. Lookups are fine; loops are not: foreach, LINQ, CopyTo, a copy into a list, and a Dictionary or HashSet held as an IEnumerable, ICollection or IReadOnlyCollection. A foreach or LINQ through IDictionary, IReadOnlyDictionary, ISet or IReadOnlySet (or their Keys and Values) is one too, since in a map only a Dictionary or a HashSet implements them." + Silence);

    public static readonly DiagnosticDescriptor NumberType = new(
        "MW0005",
        "Number type the game cannot carry",
        "'{0}' {1}",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "The game's Lua has 32-bit integers and single-precision floats. [SyncUnchecked] on the method turns this check off in its own body, except for decimal, which is wrong in the game's Lua wherever it runs.");

    public static readonly DiagnosticDescriptor Desyncs = new(
        "MW0006",
        "Desyncs wherever it runs",
        "'{0}' changes the game inside its own GetLocalPlayer() block, so it desyncs wherever it runs",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "A blizzard.j function that compares GetLocalPlayer() and then changes the game, such as SelectGroupForPlayerBJ, desyncs on every call. Do its local part inside Local.For with what the analyzer allows there." + Silence);

    public static readonly DiagnosticDescriptor UnseededRandom = new(
        "MW0038",
        "Random without a seed",
        "'{0}' seeds itself from this machine's clock and reseeds the game's random generator with it, which desyncs every random number after it, inside Local.For too; use Natives.GetRandomInt, or new Random(seed) with a seed every machine shares",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "CSharp.lua's Random without a seed calls math.randomseed(os.time()) and then math.random. In the game math.random is the game's own synced generator (seen 2026-10-26), so each machine reseeds it with its own clock, and every GetRandomInt after it differs between machines, wherever the Random was made. new Random(seed) is computed in Lua alone, so it gives the same numbers on every machine when the seed is the same (one from Natives.GetRandomInt is)." + Silence);

    public static readonly DiagnosticDescriptor Unfollowed = new(
        "MW0039",
        "Code inside Local.For the analyzer cannot follow",
        "{0}; inside Local.For, write the lambda in place or call a method of the map",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "The analyzer follows what Local.For runs: a lambda written in place, a method of the map, a delegate parameter whose callers' arguments it checks, and the delegates given to a variable, field, property or event of the map. A delegate from anywhere else (a list's element, a library's result, a lambda's parameter) runs code it cannot see, which may change the game on one machine only." + Silence);

    public static readonly DiagnosticDescriptor OneMachineCode = new(
        "MW0040",
        "Code of one machine where it runs on every machine",
        "'{0}' {1}",
        Category,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "Code the analyzer checks as code of one machine must not also run on every machine, where its machine-local reads desync the game: a delegate parameter given to Local.For (whose callers' lambdas are checked as blocks) called outside it, and [MachineLocal] on an override or an interface implementation (Moonwell calls OnGameStart on every machine) or on a lambda given to a method or event that runs it (Timer.Every). [MachineLocal] on an override of a map method that has it too is right." + Silence);

    public static ImmutableArray<DiagnosticDescriptor> All { get; } =
        ImmutableArray.Create(StateInLocal, LocalOutside, HandleId, HashLoop, NumberType, Desyncs, UnseededRandom, Unfollowed, OneMachineCode);

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
        description: "The enums of Moonwell.Objects (ArmorType, Targets, ...) are ignored by CSharp.lua: the game's Lua has their numbers only. ToString (also x?.ToString()), a string joined (+ or +=) or interpolated with one, a list or array of them given to string.Join or string.Concat, one given to an object parameter (string.Format, string.Concat, StringBuilder.Append; Equals is left alone), and Enum methods given one (GetValues, GetNames, GetName, IsDefined, Parse, TryParse, Format, ToObject) give the number where .NET gives the name, or stop. Comparing, |, & and HasFlag work.");

    public static readonly DiagnosticDescriptor HandleCheck = new(
        "MW0035",
        "Type check of a handle",
        "'{0}' checks that a value is a {1}, which the game's Lua cannot: there a cast throws and is, as or a pattern gives false or null, even for a {1}; keep the handle in a field or variable of type {1}, or in a class of your own",
        LuaCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "CSharp.lua checks a cast, is, as or a type pattern against the value's CSharp.lua class. A game handle is the game's own value (userdata), which has no such class, so a check for a handle type (unit, lightning, or a type parameter limited to one) never passes in the game: a cast throws an InvalidCastException, and is, as and patterns give false or null. A conversion to a handle type the value already is (unit to widget) checks nothing and works. The same checks hide in foreach (unit u in widgets), which casts each item, in OfType<unit>(), which tests each item with is, and in Cast<unit>() unless the items already are units. The Lua cases do not show it: there the fake world's handles are CSharp.lua objects.");

    /// <summary>The rules of C# that CSharp.lua cannot compile.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Lua { get; } = ImmutableArray.Create(StaticNatives, ChoiceNames, HandleCheck);

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

    public static readonly DiagnosticDescriptor UnseenReaction = new(
        "MW0036",
        "Reaction a package never calls",
        "'{0}' {1}",
        "Reactions",
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "The damage, input and physics packages' interfaces give every reaction an empty body, so the compiler accepts a class that only seems to write one. A package calls a reaction that is a public, non-static method of the interface's name and parameters: on a unit's class or its components (buffs too) for IDamageSource, IDamageTarget, IPushTarget and IMissileTarget, and on the game's and players' components for IDamageWatch and IMouseWatch. The rule rejects a method named like a reaction that is not one (not public, static, other parameters, an explicit implementation, or a reaction of an interface the class does not implement), a name renamed in Moonwell 0.3 (or IPushTarget's OnLanded, which is OnGrounded), a public method shaped like a reaction (a Hit, a Burst, an Impact or a Slam, or the parameter a physics reaction takes; a Mouse and a button) whose name is a letter off a reaction (one added, removed, changed, or two swapped) in a class with reactions, a reaction in a class that lacks its interface (an abstract class is left alone: its subclasses may list the interface), and an interface on a class the package never calls. For IMouseWatch an explicit implementation and a method with other parameters run in the game's Lua, which calls them by name, and not on .NET, so the two disagree. #pragma warning disable MW0036 keeps a helper's name.");

    public static readonly DiagnosticDescriptor HiddenReaction = new(
        "MW0041",
        "Reaction that never runs",
        "'{0}' {1}",
        "Reactions",
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "A reaction of a Moonwell class (a unit's OnDamaged, a component's OnKill, a game component's OnGameStart, a buff's OnTick, an area's OnUnitEntered) is a virtual method the map overrides. One written without override, or with new, hides it: the compiler only warns (CS0114), and Moonwell never calls it. A method named On... like a reaction but with other parameters never runs as the reaction either, and on .NET Moonwell's lookup of reactions by name fails on it when the class is added, while the game's Lua takes it.");

    /// <summary>The rules for reactions: the packages' interfaces (damage, mouse, physics) and Moonwell's virtual methods.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Reactions { get; } = ImmutableArray.Create(UnseenReaction, HiddenReaction);

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
        description: "The game has no setter for a unit's level, so Level is a unit's declared value: it is set in the constructor of the class that declares the unit (and in a local function there), and only read anywhere else; a lambda there may run later in the game, where the set does nothing. A hero's level is set with SetLevel; a hero's constructor does not declare it. The build rejects the same set.");

    public static readonly DiagnosticDescriptor DefaultKind = new(
        "MW0037",
        "Default<T>() of a class of another kind",
        "{0}",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "Default<T>() is declared on Widget, so C# accepts any widget class through Unit, Hero or Item: Item.Default<Guard>() compiles for a unit class, and a destructable class compiles and throws in the game. The rule rejects a class that is not of the kind the call is written on (a unit class through Item or Hero, an item class through Unit), and a destructable class anywhere, which has no default object.");

    /// <summary>The rules for the constructor of a declared object class, and for a class's default object.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Declared { get; } = ImmutableArray.Create(LiveInDeclaration, LevelOutsideConstructor, DefaultKind);

    public static readonly DiagnosticDescriptor NoId = new(
        "MW0013",
        "A declared object without its id",
        "{0} has {1}; moonwell build names a free one",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "Every class that declares an object of the map's own needs its id, as World Editor shows it, in [ObjectId(\"h001\")]; a [Replaces] class has its standard object's. A buff also gives the id of its carrier, the hidden ability the build makes to show its icon, so no class of the map declares that id: [ObjectId(\"B001\", Carrier = \"A001\")]. The build fails without them and names a free id to use.");

    public static readonly DiagnosticDescriptor BadId = new(
        "MW0014",
        "An object id the game cannot take",
        "{0} {1}",
        ObjectsCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "An object id is four ASCII letters, digits or signs, but not a comma, which splits a list field that names the object. The game makes a unit whose id starts with an upper-case letter a hero, and only such a unit: a hero's id starts with one, another unit's does not.");

    /// <summary>The rules for a declared object's id.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Ids { get; } = ImmutableArray.Create(NoId, BadId);

    private static DiagnosticDescriptor Syntax(string id, string title, string message, string description) =>
        new(id, title, message, LuaCategory, DiagnosticSeverity.Error, isEnabledByDefault: true, description: description);

    public static readonly DiagnosticDescriptor PatternStops = Syntax(
        "MW0017",
        "Pattern the compiler stops on",
        "{0}: CSharp.lua stops on this pattern",
        "CSharp.lua cannot compile these patterns: at the top of a switch expression arm a relational pattern (> 10), a type pattern without a name (int =>), var, not and a pattern in parentheses; anywhere a var or a declaration pattern inside a property pattern ({ Y: int y }), and a list pattern's slice with a name ([var first, .. var rest]). Constants, or, a typed name (int i =>) with when, property patterns, not and parentheses inside them, and parentheses in an is test work. Write a when clause on a named variable, or an if.");

    public static readonly DiagnosticDescriptor PatternWrong = Syntax(
        "MW0018",
        "Pattern that gives the wrong answer",
        "{0}",
        "A positional pattern outside a switch expression arm stops the compiler, or is false, on a value that can be null (a class, a record, a nullable struct, object: p is (1, 2)), inside another pattern ({ T: (1, _) }) and with names ((A: 1, B: 2)). On a tuple or a struct it works (t is (1, _), s is (var a, var b)). A positional pattern in a switch expression arm works, except on a class or record given null, whose Deconstruct runs before the null check (the analyzer does not see that). An extended property pattern ({ Inner.X: 9 }) works.");

    public static readonly DiagnosticDescriptor StructCopy = Syntax(
        "MW0020",
        "Struct element changed through a copy",
        "'{0}' is not a copy in the game's Lua: changing it changes the element of the array or collection; make a new struct, or write the copy back",
        "CSharp.lua does not copy a struct read from an array, a List or a Dictionary element, so changing the copy changes the element: var c = arr[0]; then c.X = 6, c.Inner.X = 6, c.Move() (a method of the struct that sets a field) or Bump(ref c). A copy from a local, a field, a property or a parameter is right, and so is a copy stored back into the same element after its changes (arr[0] = c) with no other use of that element in between.");

    public static readonly DiagnosticDescriptor TupleForm = Syntax(
        "MW0022",
        "Tuple form that fails in the game's Lua",
        "{0}",
        "var (a, b) = (1, 2) gives a the tuple; (a[0], a[1]) = (a[1], a[0]) is a Lua syntax error; and turning a tuple into text (Console.WriteLine(t), \"t=\" + t, $\"{t}\", t.ToString()) fails in ValueTuple.ToString. Write the types ((int a, int b) = (1, 2)), a method's result, or the items.");

    public static readonly DiagnosticDescriptor FormWrong = Syntax(
        "MW0023",
        "Declaration CSharp.lua writes wrong",
        "{0}",
        "A user-defined ++, --, operator true or operator false; a static abstract or static virtual member of an interface; a record that derives from a record (its == fails and its ToString leaves out the base's members); a class that implements two constructions of one generic interface (IG<int> and IG<string>); and with on an anonymous type.");

    public static readonly DiagnosticDescriptor RangeIndex = Syntax(
        "MW0024",
        "Range or index",
        "'{0}' needs System.Range or System.Index, which the game's Lua does not have; use Substring, Skip/Take or an index like list[list.Count - 1]",
        "a[1..3], s[1..3], list[^1] and the types Index and Range are missing. a[^1] on an array works.");

    public static readonly DiagnosticDescriptor RefForm = Syntax(
        "MW0025",
        "ref return or ref local of an element",
        "{0}",
        "A ref return and ref int r = ref First(arr) give a copy, so writes through r are lost; ref int r = ref a[i] and ref to a Span element are Lua syntax errors. ref parameters, out, in and ref to a local or field work.");

    public static readonly DiagnosticDescriptor ParamsForm = Syntax(
        "MW0026",
        "params that is not an array",
        "{0}",
        "params List<int> (C# 13) stops the compiler; params in a lambda is a Lua error. params int[] works.");

    public static readonly DiagnosticDescriptor StringCall = Syntax(
        "MW0027",
        "String call the game's Lua gets wrong",
        "'{0}' {1}",
        "s.Split(\"--\") with a string separator runs out of memory, and s.Split(new[] { \", \" }, options) with string separators never ends; StartsWith(char) and EndsWith(char) fail; IndexOf, StartsWith and EndsWith with an ignore-case StringComparison give the wrong answer or an error; LINQ Reverse() on a string fails. Split(char), Contains(char), the Ordinal comparisons and Replace/Equals/Compare with a comparison work.");

    public static readonly DiagnosticDescriptor BoxedValue = Syntax(
        "MW0028",
        "char or enum given to an object parameter",
        "'{0}' {1}",
        "A char or an enum given to a library method's object parameter (string.Format, string.Concat, string.Join, Console.WriteLine(object), StringBuilder.Append(object)), also through .NET 9's params ReadOnlySpan<object?> overloads, is shown as its number in the game's Lua. Joined with + or put in an interpolation hole it is right. Enums of Moonwell.Objects are MW0010.");

    public static readonly DiagnosticDescriptor CatchForm = Syntax(
        "MW0029",
        "catch that does not catch",
        "{0}",
        "A null dereference, an index outside an array and % by zero are plain Lua errors there, not NullReferenceException, IndexOutOfRangeException or DivideByZeroException: catch (Exception) catches them. An integer / by zero does throw DivideByZeroException, so that catch is rejected only around an integer %. A pattern variable in a catch filter (when (e is X x)) is nil.");

    public static readonly DiagnosticDescriptor LuaMember = Syntax(
        "MW0030",
        "Library member the game's Lua does not have",
        "'{0}' {1}",
        "The Lua core behind CSharp.lua has no such type or member, or gives another answer than .NET; the call fails when it runs, not when the map is built. The list: System.Range/Index, StringComparer, Monitor, Lock, Thread, Task, BitArray, BitOperations, HashCode, Guid, FormattableString, async/await, pointers; LINQ Append, Prepend, SkipLast, TakeLast, ToLookup, Empty, Zip, Chunk, ToHashSet, DefaultIfEmpty; List.ConvertAll; Span.ToArray/Fill/Clear; StringBuilder.Replace/Insert/Equals/AppendJoin; Exception.Source; Delegate.Target/Method; char.IsAscii/ToUpperInvariant; Environment.NewLine; Enum/Nullable.GetUnderlyingType; string.Create/Normalize/Intern/EnumerateRunes; Queue.TryPeek; Stack.TryPop/TryPeek; Dictionary.GetValueOrDefault; Convert.ToHexString; int.RotateLeft; Array.Sort(keys, items); HashSet.SetEquals (also through ISet and IReadOnlySet); Math.DivRem; Dictionary.Remove(key, out v); ToList/ToArray of a Dictionary (of a collection of the map's or Moonwell's own, such as OrderedMap, they work); Random.Shared.");

    public static readonly DiagnosticDescriptor CheckedForm = Syntax(
        "MW0031",
        "checked",
        "checked never throws in the game's Lua; test the range yourself",
        "checked(x + 1) and checked { } compile, but an overflow is never detected.");

    public static readonly DiagnosticDescriptor EnumText = Syntax(
        "MW0032",
        "Enum shown as text",
        "{0}",
        "An enum that is a [Flags] combination prints its number; Enum.Parse gives the number; Enum.GetValues and GetNames come in no fixed order; ToString(\"D\") and ToString(\"X\") on an enum give the name; and (Color)5 + \" x\" with a constant stops the compiler. Comparing, |, &, switch, TryParse and the name of a single value work.");

    public static readonly DiagnosticDescriptor UsingOrder = Syntax(
        "MW0034",
        "Two using declarations in one block",
        "using var is disposed after the earlier using var here, not before it, in the game's Lua; use nested using (...) blocks",
        "C# disposes using var declarations in reverse order; CSharp.lua disposes them in the order they are written.");

    public static readonly DiagnosticDescriptor NotANumber = Syntax(
        "MW0042",
        "NaN or infinity, which the game's Lua does not have",
        "'{0}' {1}",
        "The game's own Lua has floats of its own (the 0.3 gate, 10-27): its arithmetic makes no NaN and no infinity. float.NaN and double.NaN, which CSharp.lua writes 0 / 0, are 1 there; 1 / 0 is 2^127 (1.7e38), and inf - inf and 0 * inf are 0. So float.IsNaN is never true, IsInfinity, IsPositiveInfinity, IsNegativeInfinity and IsFinite test for what never happens, and PositiveInfinity and NegativeInfinity turn into plain numbers in arithmetic. Mark no value with a bool or a float?, use float.MaxValue for no limit, and test what could go wrong before it does (a length of 0 before dividing by it).");

    /// <summary>The rules for syntax and library CSharp.lua or its Lua core cannot do.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Syntaxes { get; } = ImmutableArray.Create(
        PatternStops, PatternWrong, StructCopy, TupleForm, FormWrong, RangeIndex, RefForm,
        ParamsForm, StringCall, BoxedValue, CatchForm, LuaMember, CheckedForm, EnumText, UsingOrder, NotANumber);
}
