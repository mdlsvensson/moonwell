using System;
using System.Linq;
using System.Reflection;

namespace Moonwell.Tests;

public class PrivateFieldTests
{
    // In Lua an object's fields and its base classes' fields share one table. CSharp.lua gives a subclass's field a name of its
    // own only when it sees the base's field of that name, and it does not see a private one, so a map's class with a field
    // named as a private field of its base would share it, with no error. A class a map can derive from has no private
    // instance field; an auto-property's is written under the property's name, which CSharp.lua sees. The Lua cases
    // shadowed-fields.* show the clash for the core's fields. CSharp.lua 2.0.4-moonwell.11 sees a base's private fields too;
    // Area keeps its private ones, so the Lua case audit.areas.shadowed-fields shows that fix holding.
    [Fact]
    public void DerivableTypesHaveNoPrivateFields()
    {
        var fields = Packages.All
            .SelectMany(assembly => assembly.GetTypes())
            .Where(type => type.IsClass && type.IsVisible && !type.IsSealed && type != typeof(Moonwell.Area))
            .Where(type => type.GetConstructors(BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic)
                .Any(constructor => constructor.IsPublic || constructor.IsFamily || constructor.IsFamilyOrAssembly))
            .SelectMany(type => type.GetFields(BindingFlags.Instance | BindingFlags.NonPublic | BindingFlags.DeclaredOnly))
            .Where(field => field.IsPrivate && !field.Name.EndsWith("k__BackingField", StringComparison.Ordinal))
            .Select(field => field.DeclaringType!.FullName + "." + field.Name)
            .OrderBy(name => name, StringComparer.Ordinal)
            .ToList();
        Assert.True(fields.Count == 0, "private fields of classes a map can derive from: " + string.Join(", ", fields));
    }
}
