using System.Text;

namespace Moonwell.Generator.Objects;

/// <summary>Makes every file of <c>Moonwell.Objects/Generated</c> from the game's export.</summary>
public static class GeneratedObjects
{
    private const string RootNamespace = "Moonwell.Objects.";

    /// <summary>The file name of a kind's values (<c>unit.json</c>), which is also the end of its embedded resource name.</summary>
    public static string ValuesFile(Kind kind)
    {
        return "Values/" + kind.ToString().ToLowerInvariant() + ".json";
    }

    /// <summary>
    /// The files, by path under <c>Generated/</c> with <c>/</c> separators: the enums of the choice types
    /// (<c>Choices.g.cs</c>), the eight kind bases (<c>Kinds/</c>), one file for each namespace (<c>Units/Human.g.cs</c>,
    /// <c>Items.g.cs</c>) and a values file for each kind (<c>Values/unit.json</c>, heroes included).
    /// </summary>
    /// <param name="tables">The export.</param>
    /// <param name="runtime">The runtime classes' members, whose names no field takes; null for none.</param>
    /// <param name="gameVersion">The game version written in each C# file's first line.</param>
    public static IReadOnlyDictionary<string, string> Make(ExportTables tables, RuntimeMembers? runtime, string gameVersion)
    {
        var files = new SortedDictionary<string, string>(StringComparer.Ordinal);
        // CSharp.lua compiles with MOONWELL_LUA and reads the build's stubs of the used classes instead.
        var header = ObjectsWriter.Header(gameVersion) + "\n#if !MOONWELL_LUA\n\n#nullable enable\n";
        const string footer = "\n#endif\n";
        var fieldsOf = new Dictionary<Kind, IReadOnlyList<Field>>();
        var objectsOf = new Dictionary<Kind, IReadOnlyList<StandardObject>>();
        var everything = new List<StandardObject>();
        var choices = Choices.All(tables, runtime?.Taken());

        // The enums are not left out under MOONWELL_LUA: the build's stubs name them. CSharp.lua writes no Lua for them.
        files["Choices.g.cs"] = ObjectsWriter.Header(gameVersion) + "\n" + ObjectsWriter.Choices(choices.Values);
        foreach (var kind in Enum.GetValues<Kind>())
        {
            var fields = Fields.Of(tables, kind, runtime?.Of(kind), choices);
            var objects = StandardObjects.Of(tables, kind, fields);
            CheckSingleChoices(fields, objects);
            fieldsOf[kind] = fields;
            objectsOf[kind] = objects;
            everything.AddRange(objects);
            files[ValuesFile(kind)] = ObjectsWriter.Values(objects, fields);
            files["Kinds/" + ObjectsWriter.KindBase(kind, false) + ".g.cs"] = header + "\n" + ObjectsWriter.Kind(kind, fields, false) + footer;
            if (kind == Kind.Unit)
            {
                files["Kinds/" + ObjectsWriter.KindBase(kind, true) + ".g.cs"] = header + "\n" + ObjectsWriter.Kind(kind, fields, true) + footer;
            }
        }

        var names = ClassNames.Assign(everything);
        var byNamespace = new SortedDictionary<string, List<(StandardObject, ClassName)>>(StringComparer.Ordinal);
        foreach (var obj in everything)
        {
            var name = names[(obj.Kind, obj.Id)];
            if (!byNamespace.TryGetValue(name.Outer, out var list))
            {
                byNamespace[name.Outer] = list = new List<(StandardObject, ClassName)>();
            }

            list.Add((obj, name));
        }

        var constructors = new StandardConstructors(fieldsOf, everything.Select(obj => (obj, names[(obj.Kind, obj.Id)])));
        foreach (var (ns, classes) in byNamespace)
        {
            files[PathOf(ns)] = header + "\n" + ObjectsWriter.Group(ns, classes, fieldsOf[Kind.Ability], constructors) + footer;
        }

        return files;
    }

    /// <summary>
    /// Fails when a standard value of a field of a single choice is a comma list: its type holds any of its values, and
    /// belongs with the list types of <see cref="Choices"/>.
    /// </summary>
    private static void CheckSingleChoices(IReadOnlyList<Field> fields, IReadOnlyList<StandardObject> objects)
    {
        var single = new Dictionary<string, Field>(StringComparer.Ordinal);
        foreach (var field in fields)
        {
            if (field.Choice is { Form: ChoiceForm.One })
            {
                single[field.Id] = field;
            }
        }

        foreach (var obj in objects)
        {
            foreach (var values in new[] { obj.Values }.Concat(obj.Sets.Values))
            {
                foreach (var (key, value) in values)
                {
                    var colon = key.IndexOf(':');
                    if (value.Contains(',') && single.TryGetValue(colon < 0 ? key : key.Substring(0, colon), out var field))
                    {
                        throw new InvalidOperationException(
                            $"{obj.Id} {key} is '{value}', a list, and {field.Choice!.Type} is a single choice; list the type in Choices");
                    }
                }
            }
        }
    }

    /// <summary>The file of a group and kind: <c>Moonwell.Objects.Human.Units</c> is <c>Human/Units.g.cs</c>.</summary>
    private static string PathOf(string ns)
    {
        if (!ns.StartsWith(RootNamespace, StringComparison.Ordinal))
        {
            throw new InvalidOperationException($"namespace {ns} is not under {RootNamespace}");
        }

        return new StringBuilder(ns.Substring(RootNamespace.Length)).Replace('.', '/').Append(".g.cs").ToString();
    }
}
