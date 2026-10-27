using System.Reflection;

namespace Moonwell.Tests;

/// <summary>The assemblies of Moonwell's core and of every package a map can reference, for the checks over their public types.</summary>
internal static class Packages
{
    public static readonly Assembly[] All =
    {
        typeof(Moonwell.Unit).Assembly,
        typeof(Moonwell.Hit).Assembly,
        typeof(Moonwell.Gizmo).Assembly,
        typeof(Moonwell.Maths).Assembly,
        typeof(Moonwell.Terrain).Assembly,
        typeof(Moonwell.Area).Assembly,
        typeof(Moonwell.Mouse).Assembly,
    };
}
