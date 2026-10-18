using System.Reflection;

namespace Moonwell.Generator.Objects;

/// <summary>
/// The names a generated property may not have: the members a standard class inherits from the runtime class of its
/// kind (<c>Moonwell.Unit</c>, <c>Moonwell.Ability</c>) and from the classes under it, read from the built
/// <c>Moonwell.dll</c>.
/// </summary>
public sealed class RuntimeMembers
{
    private readonly Assembly _runtime;

    private RuntimeMembers(Assembly runtime)
    {
        _runtime = runtime;
    }

    /// <summary>Opens the built <c>Moonwell.dll</c> at <paramref name="path"/>; the library it needs is read from beside it.</summary>
    public static RuntimeMembers Read(string path)
    {
        if (!File.Exists(path))
        {
            throw new FileNotFoundException($"{path} is not built; run dotnet build on src/Moonwell first", path);
        }

        return new RuntimeMembers(Assembly.LoadFrom(Path.GetFullPath(path)));
    }

    /// <summary>
    /// The names of the public and protected members (methods, properties, fields, events and nested types, static
    /// ones too) that the objects of <paramref name="kind"/> inherit: those of the runtime class and of every class
    /// under it, <c>object</c>'s included. Heroes inherit from <c>Moonwell.Hero</c>, so a unit's names are those of
    /// <c>Unit</c> and <c>Hero</c>.
    /// </summary>
    public IReadOnlySet<string> Of(Kind kind)
    {
        var names = new HashSet<string>(StringComparer.Ordinal);
        foreach (var typeName in TypesOf(kind))
        {
            var type = _runtime.GetType("Moonwell." + typeName)
                ?? throw new InvalidOperationException($"{_runtime.Location} has no type Moonwell.{typeName}");
            for (Type? t = type; t != null; t = t.BaseType)
            {
                foreach (var member in t.GetMembers(
                    BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance | BindingFlags.Static | BindingFlags.DeclaredOnly))
                {
                    if (IsInherited(member))
                    {
                        names.Add(member.Name);
                    }
                }
            }
        }

        return names;
    }

    private static string[] TypesOf(Kind kind)
    {
        return kind == Kind.Unit ? new[] { "Unit", "Hero" } : new[] { kind.ToString() };
    }

    /// <summary>Whether a class in another assembly sees the member by name: public or protected, and not a constructor or accessor.</summary>
    private static bool IsInherited(MemberInfo member)
    {
        switch (member)
        {
            case ConstructorInfo:
                return false;
            case MethodInfo method:
                return !method.IsSpecialName && Visible(method.IsPublic, method.IsFamily, method.IsFamilyOrAssembly);
            case FieldInfo field:
                return Visible(field.IsPublic, field.IsFamily, field.IsFamilyOrAssembly);
            case PropertyInfo property:
                var accessors = new[] { property.GetMethod, property.SetMethod };
                foreach (var accessor in accessors)
                {
                    if (accessor != null && Visible(accessor.IsPublic, accessor.IsFamily, accessor.IsFamilyOrAssembly))
                    {
                        return true;
                    }
                }

                return false;
            case EventInfo @event:
                var adder = @event.AddMethod;
                return adder != null && Visible(adder.IsPublic, adder.IsFamily, adder.IsFamilyOrAssembly);
            case Type nested:
                return nested.IsNestedPublic || nested.IsNestedFamily || nested.IsNestedFamORAssem;
            default:
                return false;
        }
    }

    private static bool Visible(bool isPublic, bool isFamily, bool isFamilyOrAssembly)
    {
        return isPublic || isFamily || isFamilyOrAssembly;
    }
}
