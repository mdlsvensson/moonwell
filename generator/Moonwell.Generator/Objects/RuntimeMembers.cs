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

    /// <summary>
    /// The names an enum of <c>Moonwell.Objects</c> may not have: the public types of namespace <c>Moonwell</c> in
    /// <c>Moonwell.dll</c> and the Moonwell libraries it references (a map that imports both namespaces could not name
    /// the enum), and the members the objects of every kind inherit (inside a standard class, one would hide the enum).
    /// </summary>
    public IReadOnlySet<string> Taken()
    {
        var names = new HashSet<string>(StringComparer.Ordinal);
        var assemblies = new List<Assembly> { _runtime };
        foreach (var reference in _runtime.GetReferencedAssemblies())
        {
            var beside = Path.Combine(Path.GetDirectoryName(_runtime.Location)!, reference.Name + ".dll");
            if (reference.Name != null && reference.Name.StartsWith("Moonwell", StringComparison.Ordinal) && File.Exists(beside))
            {
                assemblies.Add(Assembly.LoadFrom(beside));
            }
        }

        foreach (var assembly in assemblies)
        {
            foreach (var type in assembly.GetExportedTypes())
            {
                if (type.Namespace == "Moonwell" && !type.IsNested)
                {
                    names.Add(type.Name.Split('`')[0]);
                }
            }
        }

        foreach (var kind in Enum.GetValues<Kind>())
        {
            names.UnionWith(Of(kind));
        }

        return names;
    }

    /// <summary>The members of the public enum <c>Moonwell.</c><paramref name="name"/>, by name and number in its order; null when <c>Moonwell.dll</c> has none.</summary>
    public IReadOnlyList<(string Name, int Value)>? EnumMembers(string name)
    {
        var type = _runtime.GetType("Moonwell." + name);
        if (type == null || !type.IsEnum || !type.IsPublic)
        {
            return null;
        }

        return type.GetFields(BindingFlags.Public | BindingFlags.Static)
            .OrderBy(field => field.MetadataToken)
            .Select(field => (field.Name, Convert.ToInt32(field.GetRawConstantValue(), System.Globalization.CultureInfo.InvariantCulture)))
            .ToList();
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
