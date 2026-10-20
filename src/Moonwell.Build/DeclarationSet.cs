namespace Moonwell.Build
{
    /// <summary>What a map declares: its objects, the ids of the standard classes they name, and the classes its Lua uses.</summary>
    /// <param name="Objects">The declared objects, ordered by class name.</param>
    /// <param name="StandardClassIds">The standard id of each standard class a declared list or reference names, by the class's full name.</param>
    /// <param name="Used">The declared classes, the classes of <c>Moonwell.Objects</c> the map's code names and their bases up to the kind base; each base before the classes that inherit it.</param>
    /// <param name="Support">The stubs of the choice enums and <c>[BeyondLimits]</c> that CSharp.lua compiles with the used classes (<c>ObjectStubs.Support</c>); empty when the map does not use <c>Moonwell.Objects</c>.</param>
    public sealed record DeclarationSet(
        IReadOnlyList<DeclaredObject> Objects,
        IReadOnlyDictionary<string, string> StandardClassIds,
        IReadOnlyList<UsedClass> Used,
        string Support);
}
