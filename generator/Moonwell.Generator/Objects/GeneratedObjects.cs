using System.Text;
using Moonwell.GameData;

namespace Moonwell.Generator.Objects;

/// <summary>Makes every file of <c>Moonwell.Objects/Generated</c> from the game's tables.</summary>
public static class GeneratedObjects
{
    private const string RootNamespace = "Moonwell.Objects.";

    /// <summary>The file of the gameplay constants the build reads, which is also the end of its embedded resource name.</summary>
    private const string MiscFile = "Values/misc.json";

    // The keys of miscgame.txt's [Misc] section the build reads: a hero's life per point of strength and mana per point of intelligence.
    private static readonly string[] MiscKeys = { "StrHitPointBonus", "IntManaBonus" };

    /// <summary>The file name of a kind's values (<c>unit.json</c>), which is also the end of its embedded resource name.</summary>
    public static string ValuesFile(Kind kind)
    {
        return "Values/" + kind.ToString().ToLowerInvariant() + ".json";
    }

    /// <summary>
    /// The files, by path under <c>Generated/</c> with <c>/</c> separators: the enums of the choice types
    /// (<c>Choices.g.cs</c>), the eight kind bases (<c>Kinds/</c>), one file for each namespace (<c>Units/Human.g.cs</c>,
    /// <c>Items.g.cs</c>) and a values file for each kind (<c>Values/unit.json</c>, heroes included) and the gameplay constants (<c>Values/misc.json</c>).
    /// </summary>
    /// <param name="tables">The game's tables.</param>
    /// <param name="runtime">The runtime classes' members, whose names no field takes; null for none.</param>
    /// <param name="gameVersion">The game version written in each C# file's first line.</param>
    public static IReadOnlyDictionary<string, string> Make(GameTables tables, RuntimeMembers? runtime, string gameVersion)
    {
        var files = new SortedDictionary<string, string>(StringComparer.Ordinal);
        // CSharp.lua compiles with MOONWELL_LUA and reads the build's stubs of the used classes instead.
        var header = ObjectsWriter.Header(gameVersion) + "\n#if !MOONWELL_LUA\n\n#nullable enable\n";
        const string footer = "\n#endif\n";
        var fieldsOf = new Dictionary<Kind, IReadOnlyList<Field>>();
        var objectsOf = new Dictionary<Kind, IReadOnlyList<StandardObject>>();
        var everything = new List<StandardObject>();
        var listed = Choices.All(tables, runtime?.Taken());
        foreach (var kind in Enum.GetValues<Kind>())
        {
            var fields = Fields.Of(tables, kind, runtime?.Of(kind), listed);
            fieldsOf[kind] = fields;
            objectsOf[kind] = StandardObjects.Of(tables, kind, fields);
        }

        // A bit the game's own data uses is a member of its enum, and a negative it holds is allowed past the range
        // (ruling R14): a map may write any standard value.
        var choices = Choices.WithStandardBits(listed, fieldsOf.Keys.Select(kind => (fieldsOf[kind], objectsOf[kind])).ToList());
        if (runtime != null)
        {
            foreach (var choice in choices.Values.Where(choice => choice.Core))
            {
                Choices.CheckCore(choice, runtime.EnumMembers(choice.Name));
            }
        }

        // Like the classes, the enums are left out under MOONWELL_LUA: they write no Lua, but a file of them would still
        // write an empty namespace block into every map.
        files["Choices.g.cs"] = ObjectsWriter.Header(gameVersion) + "\n#if !MOONWELL_LUA\n\n" + ObjectsWriter.Choices(choices.Values) + footer;
        foreach (var kind in Enum.GetValues<Kind>())
        {
            var objects = objectsOf[kind];
            var fields = Fields.WithStandardNegatives(fieldsOf[kind], objects)
                .Select(field => field.Choice == null ? field : field with { Choice = choices[field.Choice.Type] })
                .ToList();
            CheckSingleChoices(fields, objects);
            fieldsOf[kind] = fields;
            everything.AddRange(objects);
            files[ValuesFile(kind)] = ObjectsWriter.Values(objects, fields);
            files["Kinds/" + ObjectsWriter.KindBase(kind, false) + ".g.cs"] = header + "\n" + ObjectsWriter.Kind(kind, fields, false) + footer;
            if (kind == Kind.Unit)
            {
                files["Kinds/" + ObjectsWriter.KindBase(kind, true) + ".g.cs"] = header + "\n" + ObjectsWriter.Kind(kind, fields, true) + footer;
            }
        }

        files[MiscFile] = ObjectsWriter.Misc(MiscConstants(tables));

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

    /// <summary>The constants of <see cref="MiscKeys"/> in the base file and in each balance set the tables hold, by set (<c>base</c> for the base).</summary>
    private static Dictionary<string, IReadOnlyDictionary<string, string>> MiscConstants(GameTables tables)
    {
        var sets = new Dictionary<string, IReadOnlyDictionary<string, string>>(StringComparer.Ordinal);
        foreach (var set in new string?[] { null }.Concat(tables.Sets))
        {
            var misc = tables.Misc(set);
            var constants = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var key in MiscKeys)
            {
                constants[key] = misc.TryGetValue(key, out var value)
                    ? value
                    : throw new InvalidOperationException($"miscgame.txt of {set ?? "base"} has no {key}");
            }

            sets[set ?? "base"] = constants;
        }

        return sets;
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
