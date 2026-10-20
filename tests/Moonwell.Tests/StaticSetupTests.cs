using System;
using System.Linq;

namespace Moonwell.Tests;

public class StaticSetupTests
{
    // In Lua, CSharp.lua runs a class's static setup when the class is first indexed. A map's static Dictionary keyed by the
    // class indexes it while the script loads, before the class's own file has its imports, and the script stops. A type a
    // map can name as a type argument has no static initializer, so there is no setup to run early.
    [Fact]
    public void PublicTypesHaveNoStaticInitializer()
    {
        var types = new[] { typeof(Moonwell.Unit).Assembly, typeof(Moonwell.Hit).Assembly }
            .SelectMany(assembly => assembly.GetTypes())
            .Where(type => type.IsVisible && !(type.IsAbstract && type.IsSealed))
            .Where(type => type.TypeInitializer != null)
            .Select(type => type.FullName)
            .OrderBy(name => name, StringComparer.Ordinal)
            .ToList();
        Assert.True(types.Count == 0, "public types with a static initializer: " + string.Join(", ", types));
    }
}
