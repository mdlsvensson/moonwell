namespace Moonwell.Build
{
    /// <summary>A class the map's Lua needs from <c>Moonwell.Objects</c>, or a declared class of the map: what the objects' Lua and registrations are written from.</summary>
    /// <param name="FullName">The class's full name, such as <c>Moonwell.Objects.Units.Human.Paladin</c> or <c>Objects.Arthas</c>.</param>
    /// <param name="BaseFullName">The full name of the class it inherits, such as <c>Moonwell.Objects.StandardHero</c> or <c>Moonwell.Hero</c>.</param>
    /// <param name="Kind">The kind of its kind base: <c>unit</c>, <c>hero</c>, <c>item</c>, <c>destructable</c>, <c>doodad</c>, <c>ability</c>, <c>buff</c> or <c>upgrade</c>.</param>
    /// <param name="StandardId">The id of the standard object the class itself stands for; null for a kind base and a class of the map.</param>
    /// <param name="InObjects">Whether the class is in <c>Moonwell.Objects</c>, which CSharp.lua leaves out of the script.</param>
    /// <param name="Fields">The object fields the class declares itself that the map's assembly names, in the order it declares them; none for a class of the map.</param>
    /// <param name="Methods">The methods the class declares itself that the map's assembly names, every overload of a name included; none for a class of the map.</param>
    /// <param name="Stub">The class as CSharp.lua compiles it (<see cref="ObjectStubs"/>), with only those members; empty for a class of the map.</param>
    public sealed record UsedClass(
        string FullName,
        string BaseFullName,
        string Kind,
        string? StandardId,
        bool InObjects,
        IReadOnlyList<UsedField> Fields,
        IReadOnlyList<UsedMethod> Methods,
        string Stub);
}
