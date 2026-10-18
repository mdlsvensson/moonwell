namespace Moonwell.Build
{
    /// <summary>What a map declares: its objects, the ids of the standard classes they name, and the classes its Lua uses.</summary>
    /// <param name="Objects">The declared objects, ordered by class name.</param>
    /// <param name="StandardClassIds">The standard id of each standard class a declared list or reference names, by the class's full name.</param>
    /// <param name="Used">The declared classes, the classes of <c>Moonwell.Objects</c> the map's code names and their bases up to the kind base; each base before the classes that inherit it.</param>
    public sealed record DeclarationSet(
        IReadOnlyList<DeclaredObject> Objects,
        IReadOnlyDictionary<string, string> StandardClassIds,
        IReadOnlyList<UsedClass> Used);
}
