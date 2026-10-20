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
        "'{0}' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method",
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

    public static readonly DiagnosticDescriptor CollectionExpression = new(
        "MW0007",
        "Collection expression",
        "CSharp.lua drops a collection expression or stops on it; write an initializer such as new List<int> { 1, 2 }, new() { 1, 2 } or new[] { 1, 2 }",
        LuaCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "CSharp.lua compiles a collection expression ([1, 2]) to nothing in a return or a field, and stops with \"Compiler has a bug\" in a local, an assignment or an argument.");

    public static readonly DiagnosticDescriptor StaticNatives = new(
        "MW0008",
        "using static Moonwell.Natives",
        "using static Moonwell.Natives makes CSharp.lua stop on a handle constant (such as ATTACK_TYPE_MAGIC) and hides Moonwell.Player behind Natives.Player; write Natives.X",
        LuaCategory,
        DiagnosticSeverity.Error,
        isEnabledByDefault: true,
        description: "A handle constant used through the import makes CSharp.lua stop with a NullReferenceException, and Player.Get(0) fails with CS0119 because Natives.Player(int) wins.");

    /// <summary>The rules of C# that CSharp.lua cannot compile.</summary>
    public static ImmutableArray<DiagnosticDescriptor> Lua { get; } = ImmutableArray.Create(CollectionExpression, StaticNatives);

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
}
