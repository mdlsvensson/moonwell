using System;
using System.Collections.Generic;
using System.Reflection.Emit;
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

public class ClassMethodsShadowingNativesTests
{
    private static readonly string[] Packages =
    {
        "Moonwell", "Moonwell.Damage", "Moonwell.Terrain", "Moonwell.Interactions", "Moonwell.Areas", "Moonwell.Input",
    };

    // The same hazard as above, for every class of the six packages: CSharp.lua declares a class's methods as chunk-level
    // locals, so a method named like a native the class calls (in its own code or its lambdas') calls itself instead.
    // This reads each method's IL for the calls to Natives.
    [Fact]
    public void NoClassDeclaresAMethodNamedLikeANativeItCalls()
    {
        var found = new List<string>();
        foreach (var package in Packages)
        {
            var assembly = Assembly.Load(package);
            var called = new Dictionary<Type, HashSet<string>>();
            var all = assembly.GetTypes();
            foreach (var type in all)
            {
                var owner = type;
                while (owner.DeclaringType != null)
                {
                    owner = owner.DeclaringType;
                }
                if (!called.TryGetValue(owner, out var names))
                {
                    names = new HashSet<string>();
                    called[owner] = names;
                }
                foreach (var method in AllMethods(type))
                {
                    foreach (var name in NativesCalledBy(method))
                    {
                        names.Add(name);
                    }
                }
            }
            foreach (var type in all)
            {
                if (type.DeclaringType != null)
                {
                    continue;
                }
                foreach (var method in AllMethods(type))
                {
                    if (!method.IsSpecialName && called[type].Contains(method.Name))
                    {
                        found.Add(package + ": " + type.FullName + "." + method.Name);
                    }
                }
            }
        }
        Assert.True(found.Count == 0, "Methods named like a native their class calls:\n" + string.Join("\n", found.Distinct().OrderBy(x => x)));
    }

    private static IEnumerable<MethodBase> AllMethods(Type type)
    {
        const BindingFlags flags = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Static | BindingFlags.Instance
            | BindingFlags.DeclaredOnly;
        foreach (var method in type.GetMethods(flags))
        {
            yield return method;
        }
        foreach (var constructor in type.GetConstructors(flags))
        {
            yield return constructor;
        }
    }

    private static readonly Dictionary<short, OpCode> OpCodeTable = typeof(OpCodes)
        .GetFields(BindingFlags.Public | BindingFlags.Static)
        .Select(field => (OpCode)field.GetValue(null)!)
        .ToDictionary(code => code.Value);

    private static IEnumerable<string> NativesCalledBy(MethodBase method)
    {
        var il = method.GetMethodBody()?.GetILAsByteArray();
        if (il == null)
        {
            yield break;
        }
        var module = method.Module;
        var at = 0;
        while (at < il.Length)
        {
            short value = il[at++];
            if (value == 0xFE)
            {
                value = (short)(0xFE00 | il[at++]);
            }
            var code = OpCodeTable[value];
            if (code.OperandType == OperandType.InlineMethod)
            {
                var token = BitConverter.ToInt32(il, at);
                MethodBase? target = null;
                try
                {
                    target = module.ResolveMethod(token, method.DeclaringType?.IsGenericType == true ? method.DeclaringType.GetGenericArguments() : null,
                        method.IsGenericMethod ? method.GetGenericArguments() : null);
                }
                catch (ArgumentException)
                {
                }
                if (target?.DeclaringType == typeof(Natives))
                {
                    yield return target.Name;
                }
            }
            at += code.OperandType switch
            {
                OperandType.InlineNone => 0,
                OperandType.ShortInlineBrTarget or OperandType.ShortInlineI or OperandType.ShortInlineVar => 1,
                OperandType.InlineVar => 2,
                OperandType.InlineI8 or OperandType.InlineR => 8,
                OperandType.InlineSwitch => 4 + 4 * BitConverter.ToInt32(il, at),
                _ => 4,
            };
        }
    }
}
