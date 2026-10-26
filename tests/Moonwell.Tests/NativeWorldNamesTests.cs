using System;
using System.Linq;
using System.Reflection;

namespace Moonwell.Tests;

public class NativeWorldNamesTests
{
    // In Lua, CSharp.lua declares a class's methods as chunk-level locals, so a NativeWorld method named like the native
    // it calls would call itself instead of the native.
    [Fact]
    public void NativeWorldMethodsDoNotShareNamesWithNatives()
    {
        var world = typeof(Moonwell.Hosting.IWorld).Assembly.GetType("Moonwell.Engine.NativeWorld", throwOnError: true)!;
        var natives = typeof(Natives).GetMembers(BindingFlags.Public | BindingFlags.Static | BindingFlags.Instance)
            .Select(member => member.Name)
            .ToHashSet();
        var methods = world.GetMethods(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Static
                | BindingFlags.Instance | BindingFlags.DeclaredOnly)
            .Where(method => !method.IsSpecialName)
            .Select(method => method.Name);
        var shared = methods.Where(natives.Contains).Distinct().OrderBy(name => name).ToList();
        Assert.True(shared.Count == 0, "NativeWorld methods named like natives: " + string.Join(", ", shared));
    }

    // The damage package's native world has the same hazard.
    [Fact]
    public void NativeDamageWorldMethodsDoNotShareNamesWithNatives()
    {
        var world = typeof(Moonwell.Hosting.IDamageWorld).Assembly.GetType("Moonwell.Engine.NativeDamageWorld", throwOnError: true)!;
        var natives = typeof(Natives).GetMembers(BindingFlags.Public | BindingFlags.Static | BindingFlags.Instance)
            .Select(member => member.Name)
            .ToHashSet();
        var methods = world.GetMethods(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Static
                | BindingFlags.Instance | BindingFlags.DeclaredOnly)
            .Where(method => !method.IsSpecialName)
            .Select(method => method.Name);
        var shared = methods.Where(natives.Contains).Distinct().OrderBy(name => name).ToList();
        Assert.True(shared.Count == 0, "NativeDamageWorld methods named like natives: " + string.Join(", ", shared));
    }
}
