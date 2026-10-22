namespace Moonwell.Build
{
    /// <summary>A unit, hero or item type the source map's object files make or change, as World Editor saved it.</summary>
    /// <param name="Id">The type's id: a changed standard object's own, a new object's new one.</param>
    /// <param name="Kind">The kind: <c>unit</c>, <c>hero</c> or <c>item</c>.</param>
    /// <param name="Root">The standard object whose values the type has where it changes none: its own id, or a new object's base.</param>
    /// <param name="Changed">The values the files change, keyed and typed as <see cref="DeclaredObject.Changed"/>.</param>
    public sealed record MapType(string Id, string Kind, string Root, IReadOnlyDictionary<string, object?> Changed);
}
