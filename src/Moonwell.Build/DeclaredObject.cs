namespace Moonwell.Build
{
    /// <summary>An object a map declares as a class: what the class's constructors changed in the standard object it inherits from.</summary>
    /// <param name="ClassName">The class's full name, such as <c>Objects.Arthas</c>.</param>
    /// <param name="Kind">The kind: <c>unit</c>, <c>hero</c>, <c>item</c>, <c>destructable</c>, <c>doodad</c>, <c>ability</c>, <c>buff</c> or <c>upgrade</c>.</param>
    /// <param name="StandardRoot">The id of the standard object at the root of the class's base chain.</param>
    /// <param name="Replaces">Whether the class changes the standard object itself instead of adding a new one.</param>
    /// <param name="PinnedId">The id the class's <c>[ObjectId]</c> pins; null when none.</param>
    /// <param name="Changed">
    /// The changed values by field id, or <c>&lt;field id&gt;:&lt;level&gt;</c> for a per-level field: an <c>int</c>,
    /// <c>float</c>, <c>bool</c> or <c>string</c>; for a list the full class names in order (<c>IReadOnlyList&lt;string&gt;</c>),
    /// for a reference the full class name. An id the standard data holds that no standard class has stays as its id text.
    /// Only values that differ from the standard root's are there; a per-level field the class changed has every level.
    /// </param>
    /// <param name="References">The full names of the classes the changed lists and references hold, each once, in ordinal order.</param>
    public sealed record DeclaredObject(
        string ClassName,
        string Kind,
        string StandardRoot,
        bool Replaces,
        string? PinnedId,
        IReadOnlyDictionary<string, object?> Changed,
        IReadOnlyList<string> References);
}
