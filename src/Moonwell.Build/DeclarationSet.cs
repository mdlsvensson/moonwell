namespace Moonwell.Build
{
    /// <summary>What a map declares: its objects, and the ids of the standard classes they name.</summary>
    /// <param name="Objects">The declared objects, ordered by class name.</param>
    /// <param name="StandardClassIds">The standard id of each standard class a declared list or reference names, by the class's full name.</param>
    public sealed record DeclarationSet(
        IReadOnlyList<DeclaredObject> Objects,
        IReadOnlyDictionary<string, string> StandardClassIds);
}
