using System.Globalization;

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
            foreach (var (kind, extension) in ObjectKinds.All)
            {
                var ofKind = written.Where(one => FileKindOf(one.Kind) == kind).ToList();
                var mainName = ObjectKinds.MainName(extension);
                var skinName = ObjectKinds.SkinName(extension);
                var main = ObjectTable.Read(Path.Combine(project.MapFolder, mainName), kind);
                var skin = ObjectTable.Read(Path.Combine(project.MapFolder, skinName), kind);

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
                    Add(one, IdOf(one, ids), fields, resolver, main, skin);
                }

                if (main.Existed || !main.IsEmpty)
                {
                    main.Write(Path.Combine(stagedFolder, mainName), kind);
                }

                if (skin.Existed || !skin.IsEmpty)
                {
                    skin.Write(Path.Combine(stagedFolder, skinName), kind);
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
        private static void RemoveDeclared(ObjectTable table, string name, Dictionary<string, DeclaredObject> declared, HashSet<string> warned, TextWriter log)
        {
            table.Originals.RemoveAll(entry => declared.ContainsKey(entry.Id));
            foreach (var entry in table.News)
            {
                if (declared.TryGetValue(entry.Id, out var one) && entry.OldId != one.StandardRoot && warned.Add(entry.Id))
                {
                    log.WriteLine($@"warning: map\{name}'s {entry.Id} (based on {entry.OldId}) is replaced by {one.ClassName} (based on {one.StandardRoot})");
                }
            }

            table.News.RemoveAll(entry => declared.ContainsKey(entry.Id));
        }

        // What is left is the source map's, and no class declares it. A standard object World Editor changed is not worth a word.
        private static void WarnKept(ObjectTable main, ObjectTable skin, string mainName, string skinName, IReadOnlySet<string> standardIds, TextWriter log)
        {
            var warned = new HashSet<string>(StringComparer.Ordinal);
            foreach (var (table, name) in new[] { (main, mainName), (skin, skinName) })
            {
                foreach (var entry in table.Originals.Concat(table.News))
                {
                    if (!(entry.NewId.Length == 0 && standardIds.Contains(entry.OldId)) && warned.Add(entry.Id))
                    {
                        log.WriteLine($@"warning: map\{name} keeps {entry.Id}, which no class declares");
                    }
                }
            }
        }

        private static void Add(
            DeclaredObject one,
            string id,
            Dictionary<string, StandardData.ObjectField> fields,
            Resolver resolver,
            ObjectTable main,
            ObjectTable skin)
        {
            var newId = one.Replaces ? string.Empty : id;
            var mainEntry = new ObjectTable.Entry(one.StandardRoot, newId, main.NewUnknown(), new List<ObjectTable.Modification>());
            var skinEntry = new ObjectTable.Entry(one.StandardRoot, newId, skin.NewUnknown(), new List<ObjectTable.Modification>());

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
                (field.Skin ? skinEntry : mainEntry).Modifications.Add(new ObjectTable.Modification(fieldId, type, value, level, field.Pointer));
            }

            (one.Replaces ? main.Originals : main.News).Add(mainEntry);
            (one.Replaces ? skin.Originals : skin.News).Add(skinEntry);
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
