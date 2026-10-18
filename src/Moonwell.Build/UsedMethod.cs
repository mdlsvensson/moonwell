namespace Moonwell.Build
{
    /// <summary>A method a class of <c>Moonwell.Objects</c> declares that the map's code calls, such as <c>Set</c>.</summary>
    /// <param name="LuaName">The name CSharp.lua gives it: the method's name, then its name and 1, 2, ... for each later overload.</param>
    /// <param name="Void">Whether it returns nothing; the others give nil in the game.</param>
    public sealed record UsedMethod(string LuaName, bool Void);
}
