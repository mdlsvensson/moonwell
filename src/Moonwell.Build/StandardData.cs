using System.Globalization;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Text.Json;
using Moonwell.MapFiles;

namespace Moonwell.Build
{
    /// <summary>The game's standard objects: their fields and their values in the base tables and in each set of tables.</summary>
    public sealed class StandardData
    {
        // The name a missingFrom list gives the base tables.
        private const string BaseSet = "base";

        private static readonly string[] KindFiles = Kinds.All.Select(Kinds.Key).ToArray();

        private readonly Dictionary<string, IReadOnlyList<ObjectField>> fields;
        private readonly Dictionary<string, IReadOnlySet<string>> ids;
        private readonly Dictionary<string, StandardObject> objects;
        private readonly Dictionary<string, HeroBonus> misc;

        private StandardData(
            Dictionary<string, IReadOnlyList<ObjectField>> fields,
            Dictionary<string, IReadOnlySet<string>> ids,
            Dictionary<string, StandardObject> objects,
            Dictionary<string, HeroBonus> misc)
        {
            this.fields = fields;
            this.ids = ids;
            this.objects = objects;
            this.misc = misc;
        }

        /// <summary>No standard objects, no fields and no gameplay constants: for the build of a map that declares no objects.</summary>
        public static StandardData Empty { get; } = new StandardData(
            KindFiles.ToDictionary(kind => kind, kind => (IReadOnlyList<ObjectField>)new List<ObjectField>(), StringComparer.Ordinal),
            KindFiles.ToDictionary(kind => kind, kind => (IReadOnlySet<string>)new HashSet<string>(), StringComparer.Ordinal),
            new Dictionary<string, StandardObject>(StringComparer.Ordinal),
            new Dictionary<string, HeroBonus>(StringComparer.Ordinal));

        /// <summary>A field of an object kind, as the metadata describes it.</summary>
        /// <param name="Id">The field's four-character id.</param>
        /// <param name="Type">The field's type: <c>int</c>, <c>float</c>, <c>bool</c>, <c>string</c>, <c>objectList</c>, <c>objectRef</c> or <c>choice</c>.</param>
        /// <param name="PerLevel">Whether the field has one value for each level.</param>
        /// <param name="Pointer">The data pointer of an ability field; 0 for the others.</param>
        /// <param name="Skin">Whether the field goes in the skin file.</param>
        /// <param name="Store">How the object file stores the value: <c>int</c>, <c>real</c>, <c>unreal</c> or <c>string</c>.</param>
        /// <param name="Choice">For a field of type <c>choice</c>, the enum of <c>Moonwell.Objects</c> it holds; null for the others.</param>
        public sealed record ObjectField(string Id, string Type, bool PerLevel, int Pointer, bool Skin, string Store, Choice? Choice = null);

        /// <summary>Reads the standard values embedded in the map's copy of <c>Moonwell.Objects.dll</c>.</summary>
        /// <param name="objectsDll">The path of <c>Moonwell.Objects.dll</c>, beside the map's assembly.</param>
        /// <returns>The standard data.</returns>
        /// <exception cref="BuildException">The file does not exist or holds no standard values.</exception>
        public static StandardData Read(string objectsDll)
        {
            if (!File.Exists(objectsDll))
            {
                throw new BuildException($"the standard objects do not exist: {objectsDll}");
            }

            var context = new MapLoadContext(Path.GetDirectoryName(Path.GetFullPath(objectsDll))!);
            try
            {
                return ReadIn(context, objectsDll);
            }
            finally
            {
                context.Unload();
            }
        }

        /// <summary>The fields of an object kind; the kind <c>hero</c> has the unit fields.</summary>
        /// <param name="kind">The kind: <c>unit</c>, <c>hero</c>, <c>item</c>, <c>destructable</c>, <c>doodad</c>, <c>ability</c>, <c>buff</c> or <c>upgrade</c>.</param>
        /// <returns>The fields, in the order of the metadata.</returns>
        public IReadOnlyList<ObjectField> Fields(string kind)
        {
            return fields[FileOf(kind)];
        }

        /// <summary>The ids of the standard objects of a kind; the kind <c>hero</c> has the unit ids.</summary>
        /// <param name="kind">The kind, as for <see cref="Fields"/>.</param>
        /// <returns>The ids.</returns>
        public IReadOnlySet<string> Ids(string kind)
        {
            return ids[FileOf(kind)];
        }

        /// <summary>The ids of the standard objects of every kind.</summary>
        /// <returns>The ids.</returns>
        public IReadOnlySet<string> AllIds()
        {
            var all = new HashSet<string>(StringComparer.Ordinal);
            foreach (var kind in KindFiles)
            {
                all.UnionWith(ids[kind]);
            }

            return all;
        }

        /// <summary>The value of a field of a standard object, as the game's text.</summary>
        /// <param name="standardId">The id of the standard object; ids are unique across the kinds.</param>
        /// <param name="field">The field's id.</param>
        /// <param name="level">The level of a per-level field, counted from 1; 0 for the others.</param>
        /// <param name="set">Null for the base tables, else <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>.</param>
        /// <returns>The value in the set, else the base's; null when the object has none, or does not exist.</returns>
        public string? Value(string standardId, string field, int level, string? set)
        {
            if (!objects.TryGetValue(standardId, out var standard))
            {
                return null;
            }

            var key = level > 0 ? field + ":" + level.ToString(CultureInfo.InvariantCulture) : field;
            if (set != null && standard.Sets.TryGetValue(set, out var changed) && changed.TryGetValue(key, out var inSet))
            {
                // A set that has no value for a key says so with the empty text; it does not fall back to the base.
                return inSet.Length == 0 ? null : inSet;
            }

            return standard.Values.TryGetValue(key, out var value) ? value : null;
        }

        /// <summary>The hero bonuses of the gameplay constants (<c>units/miscgame.txt</c>) of a set of tables.</summary>
        /// <param name="set">Null for the base tables, else <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>.</param>
        /// <returns>The life each point of strength and the mana each point of intelligence adds.</returns>
        /// <exception cref="BuildException">The data holds no gameplay constants for the set.</exception>
        public HeroBonus Misc(string? set)
        {
            return misc.TryGetValue(set ?? BaseSet, out var bonus)
                ? bonus
                : throw new BuildException($"the standard data holds no gameplay constants for {set ?? BaseSet}");
        }

        /// <summary>Whether a standard object does not exist in the game under a set of tables (<see cref="GameData.ExistenceSet"/>).</summary>
        /// <param name="standardId">The id of the standard object.</param>
        /// <param name="set">Null for the base tables, else <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>.</param>
        /// <returns>True when the set's tables leave the object out; false when it exists there, and for an id that is no standard object.</returns>
        public bool IsMissing(string standardId, string? set)
        {
            return objects.TryGetValue(standardId, out var standard) && standard.MissingFrom.Contains(set ?? BaseSet);
        }

        // Kept apart from Read so that nothing of the loaded assembly is on Read's frame when the context unloads.
        [MethodImpl(MethodImplOptions.NoInlining)]
        private static StandardData ReadIn(MapLoadContext context, string objectsDll)
        {
            var assembly = context.Open(objectsDll);
            var fields = new Dictionary<string, IReadOnlyList<ObjectField>>(StringComparer.Ordinal);
            var ids = new Dictionary<string, IReadOnlySet<string>>(StringComparer.Ordinal);
            var objects = new Dictionary<string, StandardObject>(StringComparer.Ordinal);
            var choices = new Dictionary<string, Choice>(StringComparer.Ordinal);
            Choice ChoiceOf(string name)
            {
                if (!choices.TryGetValue(name, out var choice))
                {
                    // A core enum's texts (Moonwell.AttackType's) are on the internal enum of its name in CoreChoices.
                    var type = assembly.GetType("Moonwell.Objects." + name) ?? assembly.GetType("Moonwell.Objects.CoreChoices+" + name)
                        ?? throw new BuildException($"{Path.GetFileName(objectsDll)} has no enum Moonwell.Objects.{name}, which its values name");
                    choices[name] = choice = Choice.Read(type);
                }

                return choice;
            }

            foreach (var kind in KindFiles)
            {
                var resource = $"Moonwell.Objects.Values.{kind}.json";
                using var stream = assembly.GetManifestResourceStream(resource)
                    ?? throw new BuildException($"{Path.GetFileName(objectsDll)} holds no {resource}");
                using var document = JsonDocument.Parse(stream);
                fields[kind] = ReadFields(document.RootElement.GetProperty("fields"), ChoiceOf);
                var kindIds = new HashSet<string>(StringComparer.Ordinal);
                foreach (var entry in document.RootElement.GetProperty("objects").EnumerateObject())
                {
                    // Value is keyed by the id alone, which holds while no id is in two kinds (true of the game's data).
                    if (!objects.TryAdd(entry.Name, ReadObject(entry.Value)))
                    {
                        throw new BuildException($"the standard id {entry.Name} is in two object kinds; Value cannot tell them apart");
                    }

                    kindIds.Add(entry.Name);
                }

                ids[kind] = kindIds;
            }

            return new StandardData(fields, ids, objects, ReadMisc(assembly, objectsDll));
        }

        private static Dictionary<string, HeroBonus> ReadMisc(Assembly assembly, string objectsDll)
        {
            const string resource = "Moonwell.Objects.Values.misc.json";
            using var stream = assembly.GetManifestResourceStream(resource)
                ?? throw new BuildException($"{Path.GetFileName(objectsDll)} holds no {resource}");
            using var document = JsonDocument.Parse(stream);
            var misc = new Dictionary<string, HeroBonus>(StringComparer.Ordinal);
            foreach (var set in document.RootElement.EnumerateObject())
            {
                misc[set.Name] = new HeroBonus(
                    float.Parse(set.Value.GetProperty("StrHitPointBonus").GetString()!, CultureInfo.InvariantCulture),
                    float.Parse(set.Value.GetProperty("IntManaBonus").GetString()!, CultureInfo.InvariantCulture));
            }

            return misc;
        }

        private static IReadOnlyList<ObjectField> ReadFields(JsonElement array, Func<string, Choice> choiceOf)
        {
            var list = new List<ObjectField>();
            foreach (var field in array.EnumerateArray())
            {
                var choice = field.TryGetProperty("choice", out var name) ? choiceOf(name.GetString()!) : null;
                list.Add(new ObjectField(
                    field.GetProperty("id").GetString()!,
                    field.GetProperty("type").GetString()!,
                    field.GetProperty("perLevel").GetBoolean(),
                    field.GetProperty("pointer").GetInt32(),
                    field.GetProperty("skin").GetBoolean(),
                    field.GetProperty("store").GetString()!,
                    choice));
            }

            return list;
        }

        private static StandardObject ReadObject(JsonElement element)
        {
            var sets = new Dictionary<string, IReadOnlyDictionary<string, string>>(StringComparer.Ordinal);
            foreach (var set in element.GetProperty("sets").EnumerateObject())
            {
                sets[set.Name] = ReadMap(set.Value);
            }

            var missing = new HashSet<string>(StringComparer.Ordinal);
            if (element.TryGetProperty("missingFrom", out var missingFrom))
            {
                foreach (var set in missingFrom.EnumerateArray())
                {
                    missing.Add(set.GetString()!);
                }
            }

            return new StandardObject(ReadMap(element.GetProperty("values")), sets, missing);
        }

        private static Dictionary<string, string> ReadMap(JsonElement element)
        {
            var map = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var entry in element.EnumerateObject())
            {
                map[entry.Name] = entry.Value.GetString()!;
            }

            return map;
        }

        private static string FileOf(string kind)
        {
            return kind == "hero" ? "unit" : kind;
        }

        private sealed record StandardObject(
            IReadOnlyDictionary<string, string> Values,
            IReadOnlyDictionary<string, IReadOnlyDictionary<string, string>> Sets,
            IReadOnlySet<string> MissingFrom);
    }
}
