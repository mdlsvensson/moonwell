using System.Collections.Immutable;
using Microsoft.CodeAnalysis;

namespace Moonwell.Analyzers;

/// <summary>The multiplayer mistakes the analyzer rejects (decision 13). <c>[SyncUnchecked]</c> on a method turns all of them off there.</summary>
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
}
