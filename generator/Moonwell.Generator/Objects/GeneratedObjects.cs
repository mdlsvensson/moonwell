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
    /// The files, by path under <c>Generated/</c> with <c>/</c> separators: the eight kind bases (<c>Kinds/</c>), one
    /// file for each namespace (<c>Units/Human.g.cs</c>, <c>Items.g.cs</c>) and a values file for each kind
    /// (<c>Values/unit.json</c>, heroes included).
    /// </summary>
    /// <param name="tables">The export.</param>
    /// <param name="runtime">The runtime classes' members, whose names no field takes; null for none.</param>
    /// <param name="gameVersion">The game version written in each C# file's first line.</param>
    public static IReadOnlyDictionary<string, string> Make(ExportTables tables, RuntimeMembers? runtime, string gameVersion)
    {
        var files = new SortedDictionary<string, string>(StringComparer.Ordinal);
        var header = ObjectsWriter.Header(gameVersion) + "\n#nullable enable\n";
        var fieldsOf = new Dictionary<Kind, IReadOnlyList<Field>>();
        var objectsOf = new Dictionary<Kind, IReadOnlyList<StandardObject>>();
        var everything = new List<StandardObject>();
        foreach (var kind in Enum.GetValues<Kind>())
        {
            var fields = Fields.Of(tables, kind, runtime?.Of(kind));
            var objects = StandardObjects.Of(tables, kind, fields);
            fieldsOf[kind] = fields;
            objectsOf[kind] = objects;
            everything.AddRange(objects);
            files[ValuesFile(kind)] = ObjectsWriter.Values(objects, fields);
            files["Kinds/" + ObjectsWriter.KindBase(kind, false) + ".g.cs"] = header + "\n" + ObjectsWriter.Kind(kind, fields, false);
            if (kind == Kind.Unit)
            {
                files["Kinds/" + ObjectsWriter.KindBase(kind, true) + ".g.cs"] = header + "\n" + ObjectsWriter.Kind(kind, fields, true);
            }
        }

        var names = ClassNames.Assign(everything);
        var byNamespace = new SortedDictionary<string, List<(StandardObject, ClassName)>>(StringComparer.Ordinal);
        foreach (var obj in everything)
        {
            var name = names[(obj.Kind, obj.Id)];
            if (!byNamespace.TryGetValue(name.Namespace, out var list))
            {
                byNamespace[name.Namespace] = list = new List<(StandardObject, ClassName)>();
            }

            list.Add((obj, name));
        }

        foreach (var (ns, classes) in byNamespace)
        {
            files[PathOf(ns)] = header + "\n" + ObjectsWriter.Namespace(ns, classes, fieldsOf[Kind.Ability]);
        }

        return files;
    }

    /// <summary>The file of a namespace: <c>Moonwell.Objects.Units.Human</c> is <c>Units/Human.g.cs</c>.</summary>
    private static string PathOf(string ns)
    {
        if (!ns.StartsWith(RootNamespace, StringComparison.Ordinal))
        {
            throw new InvalidOperationException($"namespace {ns} is not under {RootNamespace}");
        }

        return new StringBuilder(ns.Substring(RootNamespace.Length)).Replace('.', '/').Append(".g.cs").ToString();
    }
}
