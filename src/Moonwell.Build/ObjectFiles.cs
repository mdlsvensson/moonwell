using System.Globalization;
using Moonwell.GameData;
using Moonwell.MapFiles;
using War3Net.Build.Object;

namespace Moonwell.Build
{
    /// <summary>Writes the declared objects into the staged map's object-data files, merged with the source map's own.</summary>
    public static class ObjectFiles
    {
        /// <summary>
        /// Writes the main and the skin file of each kind into the staged folder: the source map's objects, except those a
        /// declared object has the id of, then the declared objects (a <c>[Replaces]</c> one in the table of changed standard
        /// objects, the others in the table of new ones, based on their standard root) and the carrier ability of each buff.
        /// A field goes in the skin file when the metadata says so, else in the main file. A file the source map lacks is
        /// written only when it holds an object.
        /// </summary>
        /// <param name="project">The map project; its map folder holds the source map's object files.</param>
        /// <param name="objects">The declared objects.</param>
        /// <param name="ids">The object id of each declared class, and of each buff's carrier as <c>&lt;class&gt;#carrier</c>.</param>
        /// <param name="standardClassIds">The standard id of each standard class a declared list or reference names.</param>
        /// <param name="standard">The standard fields and ids.</param>
        /// <param name="stagedFolder">The staged map folder.</param>
        /// <param name="log">The build log; warnings start with <c>warning: </c>.</param>
        /// <exception cref="BuildException">A list or reference names a class that has no object, or a source file cannot be read.</exception>
        public static void Write(
            MapProject project,
            IReadOnlyList<DeclaredObject> objects,
            IReadOnlyDictionary<string, string> ids,
            IReadOnlyDictionary<string, string> standardClassIds,
            StandardData standard,
            string stagedFolder,
            TextWriter log)
        {
            var written = new List<DeclaredObject>(objects);
            written.AddRange(Carriers.Of(objects, ids));
            var resolver = new Resolver(ids, standardClassIds, standard);

            Directory.CreateDirectory(stagedFolder);
            foreach (var fileKind in Kinds.All)
            {
                var kind = Kinds.Key(fileKind);
                var ofKind = written.Where(one => FileKindOf(one.Kind) == kind).ToList();
                var mainName = Kinds.MainName(fileKind);
                var skinName = Kinds.SkinName(fileKind);
                var main = SourceObjectFile.Read(Path.Combine(project.MapFolder, mainName), fileKind);
                var skin = SourceObjectFile.Read(Path.Combine(project.MapFolder, skinName), fileKind);

                var declared = new Dictionary<string, DeclaredObject>(StringComparer.Ordinal);
                foreach (var one in ofKind)
                {
                    declared[IdOf(one, ids)] = one;
                }

                var replaced = new HashSet<string>(StringComparer.Ordinal);
                RemoveDeclared(main, mainName, declared, replaced, log);
                RemoveDeclared(skin, skinName, declared, replaced, log);
                WarnKept(main, skin, mainName, skinName, standard.Ids(kind), log);

                var fields = new Dictionary<string, StandardData.ObjectField>(StringComparer.Ordinal);
                foreach (var field in standard.Fields(kind))
                {
                    fields[field.Id] = field;
                }

                foreach (var one in ofKind)
                {
                    Add(fileKind, one, IdOf(one, ids), fields, resolver, main, skin);
                }

                if (main.Existed || !main.IsEmpty)
                {
                    main.Write(Path.Combine(stagedFolder, mainName));
                }

                if (skin.Existed || !skin.IsEmpty)
                {
                    skin.Write(Path.Combine(stagedFolder, skinName));
                }
            }
        }

        private static string FileKindOf(string kind)
        {
            return kind == "hero" ? "unit" : kind;
        }

        private static string IdOf(DeclaredObject one, IReadOnlyDictionary<string, string> ids)
        {
            return ids.TryGetValue(one.ClassName, out var id) ? id : throw new BuildException($"{one.ClassName} has no object id");
        }

        // A new object of the source map that a declared object takes the id of is replaced; a different base is worth a word.
        private static void RemoveDeclared(SourceObjectFile table, string name, Dictionary<string, DeclaredObject> declared, HashSet<string> warned, TextWriter log)
        {
            foreach (var entry in table.File.Original.Where(entry => declared.ContainsKey(SourceObjectFile.IdOf(entry))).ToList())
            {
                table.File.Remove(entry);
            }

            foreach (var entry in table.File.Custom)
            {
                var id = SourceObjectFile.IdOf(entry);
                if (declared.TryGetValue(id, out var one) && entry.BaseId != one.StandardRoot && warned.Add(id))
                {
                    log.WriteLine($@"warning: map\{name}'s {id} (based on {entry.BaseId}) is replaced by {one.ClassName} (based on {one.StandardRoot})");
                }
            }

            foreach (var entry in table.File.Custom.Where(entry => declared.ContainsKey(SourceObjectFile.IdOf(entry))).ToList())
            {
                table.File.Remove(entry);
            }
        }

        // What is left is the source map's, and no class declares it. A standard object World Editor changed is not worth a word.
        private static void WarnKept(SourceObjectFile main, SourceObjectFile skin, string mainName, string skinName, IReadOnlySet<string> standardIds, TextWriter log)
        {
            var warned = new HashSet<string>(StringComparer.Ordinal);
            foreach (var (table, name) in new[] { (main, mainName), (skin, skinName) })
            {
                foreach (var entry in table.Entries)
                {
                    var id = SourceObjectFile.IdOf(entry);
                    if (!(entry.Id.Length == 0 && standardIds.Contains(entry.BaseId)) && warned.Add(id))
                    {
                        log.WriteLine($@"warning: map\{name} keeps {id}, which no class declares");
                    }
                }
            }
        }

        /// <summary>
        /// Fails a new buff that keeps its standard root's extended tooltip when that text holds a comma: the game shows a
        /// map's own buff's extended tooltip only up to its first comma, inherited or not, where the game's own buff shows
        /// it whole (game facts, probe of 2026-10-29: a buff based on Flame Strike showed "This unit is in a Flame Strike").
        /// A text the class sets is checked as it is written.
        /// </summary>
        /// <param name="objects">The declared objects.</param>
        /// <param name="standard">The standard values.</param>
        /// <param name="set">The map's set of tables: null for the base tables, else <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>.</param>
        /// <exception cref="BuildException">A new buff inherits an extended tooltip with a comma.</exception>
        public static void CheckInheritedBuffTooltips(IEnumerable<DeclaredObject> objects, StandardData standard, string? set)
        {
            foreach (var one in objects)
            {
                if (one.Kind != "buff" || one.Replaces || one.Changed.ContainsKey("fube"))
                {
                    continue;
                }

                var inherited = standard.Value(one.StandardRoot, "fube", 0, set);
                if (inherited != null && inherited.Contains(','))
                {
                    throw new BuildException($"{one.ClassName}: fube (the extended tooltip) inherits \"{inherited}\" from {one.StandardRoot}, which holds a comma, and the game shows a map's own buff's extended tooltip only up to its first comma; set TooltipExtended to a text without one");
                }
            }
        }

        private static void Add(
            Kind fileKind,
            DeclaredObject one,
            string id,
            Dictionary<string, StandardData.ObjectField> fields,
            Resolver resolver,
            SourceObjectFile main,
            SourceObjectFile skin)
        {
            // A change to a standard object is in the original table; a new object is in the custom table, based on its standard root.
            var mainEntry = one.Replaces ? main.File.AddOriginal(one.StandardRoot) : main.File.AddCustom(one.StandardRoot, id);
            var skinEntry = one.Replaces ? skin.File.AddOriginal(one.StandardRoot) : skin.File.AddCustom(one.StandardRoot, id);
            var hasLevels = Kinds.HasLevels(fileKind);

            foreach (var key in OrderedKeys(one))
            {
                var colon = key.IndexOf(':');
                var fieldId = colon < 0 ? key : key.Substring(0, colon);
                var level = colon < 0 ? 0 : int.Parse(key.Substring(colon + 1), CultureInfo.InvariantCulture);
                if (!fields.TryGetValue(fieldId, out var field))
                {
                    throw new BuildException($"{one.ClassName}: {fieldId} is not a field of a {one.Kind}");
                }

                var (type, value) = StoredValues.Of(one.ClassName, field, one.Changed[key]!, name => resolver.IdOf(one.ClassName, fieldId, name));

                // The game shows a buff's extended tooltip only up to its first comma, quoted or not, so a placeholder
                // (<A000,Dur1>), which holds one, cannot work in it either (game facts, probe of 2026-10-29).
                if (one.Kind == "buff" && fieldId == "fube" && value is string tooltip && tooltip.Contains(','))
                {
                    throw new BuildException($"{one.ClassName}: fube (the extended tooltip) holds a comma, and the game shows a buff's extended tooltip only up to its first comma; a placeholder such as <A000,Dur1> holds one too");
                }

                // A unit's, item's, destructable's or buff's modification has no level and no pointer in the file. Set changes a
                // modification of the same field and level instead of adding another; the declared keys (a field and a level) never collide.
                try
                {
                    (field.Skin ? skinEntry : mainEntry).Set(fieldId, hasLevels ? level : 0, hasLevels ? field.Pointer : 0, FieldValueOf(type, value));
                }
                catch (ArgumentException failure) when (failure.ParamName == "Value")
                {
                    // An object file stores a text NUL-terminated as UTF-8: no NUL, no half of a surrogate pair.
                    var reason = value is string text && text.Contains('\0')
                        ? "a NUL character, which an object file cannot store"
                        : "text that is not valid Unicode (half of a surrogate pair)";
                    throw new BuildException($"{one.ClassName}: {fieldId} holds {reason}");
                }
            }
        }

        // The type and value StoredValues gives, as Moonwell.MapFiles holds them.
        private static FieldValue FieldValueOf(ObjectDataType type, object value)
        {
            return type switch
            {
                ObjectDataType.Int => new FieldValue.Int((int)value),
                ObjectDataType.Real => new FieldValue.Real((float)value),
                ObjectDataType.Unreal => new FieldValue.Unreal((float)value),
                ObjectDataType.String => new FieldValue.Text((string)value),
                _ => throw new InvalidOperationException($"no stored value of type {type}"),
            };
        }

        // By field, then by level as a number.
        private static IEnumerable<string> OrderedKeys(DeclaredObject one)
        {
            return one.Changed.Keys
                .Where(key => one.Changed[key] != null)
                .OrderBy(key => key.Split(':')[0], StringComparer.Ordinal)
                .ThenBy(key => key.Contains(':') ? int.Parse(key.Split(':')[1], CultureInfo.InvariantCulture) : 0);
        }

        // The object id a class name in a list or a reference stands for.
        private sealed class Resolver
        {
            private readonly IReadOnlyDictionary<string, string> ids;
            private readonly IReadOnlyDictionary<string, string> standardClassIds;
            private readonly HashSet<string> standardIds = new HashSet<string>(StringComparer.Ordinal);

            public Resolver(IReadOnlyDictionary<string, string> ids, IReadOnlyDictionary<string, string> standardClassIds, StandardData standard)
            {
                this.ids = ids;
                this.standardClassIds = standardClassIds;
                standardIds.UnionWith(standard.AllIds());
            }

            public string IdOf(string owner, string field, string name)
            {
                if (ids.TryGetValue(name, out var declared))
                {
                    return declared;
                }

                if (standardClassIds.TryGetValue(name, out var standard))
                {
                    return standard;
                }

                // An id no standard class has stays as its id text, and so does an id of the standard data that names no
                // object (a tavern hero requires TALT), as FieldValues.IdOf lets it through: four characters, no class name.
                if (standardIds.Contains(name) || (name.Length == 4 && !name.Contains('.')))
                {
                    return name;
                }

                throw new BuildException($"{owner}: {field} names {name}, which no object declares");
            }
        }
    }
}
