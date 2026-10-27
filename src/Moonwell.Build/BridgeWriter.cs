using System.Globalization;
using System.Text;
using War3Net.Build.Widget;

namespace Moonwell.Build
{
    /// <summary>
    /// Writes the map's placed bridges, read from <c>war3map.doo</c>, as the C# file <c>Moonwell.Physics</c> makes its decks
    /// from: each placed walkable destructable (its type, the number of its deck shape, its place, facing and scale, and
    /// whether its type turns with its facing) and the deck shapes of the types placed, from the shapes Moonwell ships
    /// (<c>Data/Decks.txt</c>, measured in the game). A type of the map's own that is based on a standard bridge takes its
    /// base's shape, unless it changes the model or the fixed rotation, which would change the deck; one declared in code
    /// wins over the object files. A placed walkable type with no shape is a warning, as is a placed type that is neither
    /// standard nor the map's own: the package does not see it as ground. One placed dead (life 0) is left out.
    /// </summary>
    public static class BridgeWriter
    {
        private const string FileName = "PhysicsData.g.cs";

        private const string ShapesResource = "Moonwell.Build.Data.Decks.txt";

        // The fields of a destructable that change its deck: the model, the fixed rotation; and whether it is walkable.
        private const string ModelField = "bfil";

        private const string RotationField = "bfxr";

        private const string WalkableField = "bwal";

        // The walkable types whose fixed rotation is -1, which the game turns by their facing in war3map.doo; every other
        // walkable type keeps its fixed rotation, as its shape was measured (Moonwell.Objects' standard data, 3.0.1).
        private static readonly string[] Turning = { "DTep", "DTs0", "DTs1", "DTs2", "DTs3", "ITf1", "ITf2", "ITf3", "ITf4", "WGTR" };

        /// <summary>The placed bridges' text, one bridge a line: <c>type shape x y facing scale turns</c>.</summary>
        /// <param name="project">The map project; its map folder's object files give the map's own destructable types.</param>
        /// <param name="doodads">The map's placed doodads and destructables; null when it has no <c>war3map.doo</c>.</param>
        /// <param name="declared">The destructable types the map declares in code, which win over its object files; empty without <c>Moonwell.Objects</c>.</param>
        /// <param name="standardIds">The ids of the game's standard objects, of every kind; null without <c>Moonwell.Objects</c>, when a placed type the map does not know cannot be told from a standard one.</param>
        /// <param name="log">Where a placed walkable type with no deck shape, and a placed type nobody knows, are warned of.</param>
        /// <param name="decks">The shapes of the types placed, in the order the bridges number them: <c>cell columns rows originColumn originRow</c>, then the rows.</param>
        /// <returns>The text.</returns>
        public static string Encode(MapProject project, MapDoodads? doodads, IReadOnlyList<OwnType> declared, IReadOnlySet<string>? standardIds, TextWriter log, out string decks)
        {
            var shapes = Shapes(out var unmeasured);
            var own = new Dictionary<string, OwnType>(StringComparer.Ordinal);
            ReadObjects(project, own);
            foreach (var type in declared)
            {
                own[type.Id] = type;
            }

            var bridges = new StringBuilder();
            var used = new List<string>();
            var warned = new HashSet<string>(StringComparer.Ordinal);
            foreach (var doodad in doodads?.Doodads ?? new List<DoodadData>())
            {
                // One placed dead is not there to stand on at the start.
                if (doodad.Life == 0)
                {
                    continue;
                }

                var id = ObjectTable.TextOf(doodad.TypeId);
                own.TryGetValue(id, out var mine);
                var baseType = mine?.Base ?? id;
                var variation = doodad.Variation.ToString(CultureInfo.InvariantCulture);
                var key = baseType + " " + variation;
                var standard = shapes.ContainsKey(key) || unmeasured.Contains(baseType) || shapes.Keys.Any(shape => shape.StartsWith(baseType + " ", StringComparison.Ordinal));
                if (mine == null && !standard && standardIds != null && !standardIds.Contains(id))
                {
                    if (warned.Add(id))
                    {
                        log.WriteLine($"warning: war3map.doo places {id}, a type that is neither a standard object nor one of the map's own: if it is a bridge, the physics package does not see it as ground");
                    }

                    continue;
                }

                if (!(mine?.Walkable ?? standard))
                {
                    continue;
                }

                var reshaped = mine != null && mine.Reshaped;
                if (!shapes.ContainsKey(key) || reshaped)
                {
                    if (warned.Add(id + " " + variation))
                    {
                        log.WriteLine($"warning: war3map.doo places {Described(id, variation, mine)}, a walkable destructable with no deck shape: the physics package does not see it as ground");
                    }

                    continue;
                }

                var index = used.IndexOf(key);
                if (index < 0)
                {
                    index = used.Count;
                    used.Add(key);
                }

                var facing = doodad.Rotation * 180.0 / Math.PI;
                bridges.Append(id)
                    .Append(' ').Append(index.ToString(CultureInfo.InvariantCulture))
                    .Append(' ').Append(Number(doodad.Position.X))
                    .Append(' ').Append(Number(doodad.Position.Y))
                    .Append(' ').Append(Number(facing))
                    .Append(' ').Append(Number(doodad.Scale.X))
                    .Append(' ').Append(Array.IndexOf(Turning, baseType) >= 0 ? '1' : '0')
                    .Append('\n');
            }

            var text = new StringBuilder();
            foreach (var key in used)
            {
                text.Append(shapes[key]);
            }

            decks = text.ToString();
            return bridges.ToString();
        }

        /// <summary>The destructable types a map declares in code, as the bridges see them, and its doodad types, which are never walkable.</summary>
        /// <param name="objects">The map's declared objects.</param>
        /// <param name="ids">The id of each declared class, by its full name.</param>
        /// <returns>The declared destructables.</returns>
        public static IReadOnlyList<OwnType> Declared(IReadOnlyList<DeclaredObject> objects, IReadOnlyDictionary<string, string> ids)
        {
            var types = new List<OwnType>();
            foreach (var declared in objects)
            {
                if ((declared.Kind != "destructable" && declared.Kind != "doodad") || !ids.TryGetValue(declared.ClassName, out var id))
                {
                    continue;
                }

                if (declared.Kind == "doodad")
                {
                    // A doodad is never walkable; it is listed so that placing one is not a type nobody knows.
                    types.Add(new OwnType(id, declared.StandardRoot, false, false, declared.ClassName));
                    continue;
                }

                bool? walkable = declared.Changed.TryGetValue(WalkableField, out var value) && value is bool set ? set : null;
                var reshaped = declared.Changed.ContainsKey(ModelField) || declared.Changed.ContainsKey(RotationField);
                types.Add(new OwnType(id, declared.StandardRoot, reshaped, walkable, declared.ClassName));
            }

            return types;
        }

        /// <summary>
        /// Writes <c>PhysicsData.g.cs</c> into the project's generated folder: <c>Moonwell.Generated.PhysicsData</c> with the bridges'
        /// text from <c>Bridges()</c> and the shapes' from <c>Decks()</c>, methods so that CSharp.lua writes each text once.
        /// </summary>
        /// <param name="project">The map project.</param>
        /// <param name="doodads">The map's placed doodads and destructables; null when it has no <c>war3map.doo</c>.</param>
        /// <param name="declared">The destructable types the map declares in code; empty without <c>Moonwell.Objects</c>.</param>
        /// <param name="standardIds">The ids of the game's standard objects; null without <c>Moonwell.Objects</c>.</param>
        /// <param name="log">Where a placed walkable type with no deck shape, and a placed type nobody knows, are warned of.</param>
        /// <returns>The path of the written file.</returns>
        public static string Write(MapProject project, MapDoodads? doodads, IReadOnlyList<OwnType> declared, IReadOnlySet<string>? standardIds, TextWriter log)
        {
            var bridges = Encode(project, doodads, declared, standardIds, log, out var decks);
            Directory.CreateDirectory(project.GeneratedFolder);
            var path = Path.Combine(project.GeneratedFolder, FileName);
            var text = string.Join(
                "\n",
                "// Generated by Moonwell's build from war3map.doo and the deck shapes it ships. Do not edit; every build writes it anew.",
                "namespace Moonwell.Generated",
                "{",
                "    public static class PhysicsData",
                "    {",
                "        public static string Bridges() => \"" + Escaped(bridges) + "\";",
                "        public static string Decks() => \"" + Escaped(decks) + "\";",
                "    }",
                "}",
                string.Empty);
            File.WriteAllText(path, text);
            return path;
        }

        /// <summary>Removes the file an earlier build wrote, which a map that no longer references the package would not compile.</summary>
        /// <param name="project">The map project.</param>
        public static void Delete(MapProject project)
        {
            var path = Path.Combine(project.GeneratedFolder, FileName);
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }

        // The shipped shapes, keyed by "type variation", each as the package reads it: its header without the type and
        // variation, then its rows, a line each. The walkable types the file leaves out are listed in its "# Left out" lines.
        private static Dictionary<string, string> Shapes(out HashSet<string> unmeasured)
        {
            var shapes = new Dictionary<string, string>(StringComparer.Ordinal);
            unmeasured = new HashSet<string>(StringComparer.Ordinal);
            using var stream = typeof(BridgeWriter).Assembly.GetManifestResourceStream(ShapesResource)
                ?? throw new InvalidOperationException("the build has no " + ShapesResource);
            using var reader = new StreamReader(stream);
            var lines = new List<string>();
            string? line;
            while ((line = reader.ReadLine()) != null)
            {
                lines.Add(line);
            }

            for (var i = 0; i < lines.Count; i++)
            {
                var current = lines[i];
                if (current.StartsWith("# Left out", StringComparison.Ordinal))
                {
                    // "# Left out, why (n): ITf1 0, ITf2 0 (more)"
                    var list = current.Substring(current.IndexOf(':') + 1).Split('(')[0];
                    foreach (var one in list.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries))
                    {
                        unmeasured.Add(one.Split(' ')[0]);
                    }

                    continue;
                }

                if (current.Length == 0 || current[0] == '#')
                {
                    continue;
                }

                var header = current.Split(' ', StringSplitOptions.RemoveEmptyEntries);
                var rows = int.Parse(header[4], CultureInfo.InvariantCulture);
                var shape = new StringBuilder();
                shape.Append(string.Join(' ', header, 2, 5)).Append('\n');
                for (var row = 1; row <= rows; row++)
                {
                    shape.Append(lines[i + row]).Append('\n');
                }

                shapes[header[0] + " " + header[1]] = shape.ToString();
                i += rows;
            }

            return shapes;
        }

        // The map's own destructable types (new ones, and standard ones it changes), from its object files, main and skin:
        // each one's base, whether it changes the model or the fixed rotation, and whether it sets itself walkable.
        private static void ReadObjects(MapProject project, Dictionary<string, OwnType> types)
        {
            // The map's own doodads are never walkable; they are listed so that placing one is not a type nobody knows.
            foreach (var name in new[] { ObjectKinds.MainName("w3d"), ObjectKinds.SkinName("w3d") })
            {
                var table = ObjectTable.Read(Path.Combine(project.MapFolder, name), "doodad");
                foreach (var entry in table.Originals.Concat(table.News))
                {
                    types[entry.Id] = new OwnType(entry.Id, entry.OldId, false, false, null);
                }
            }

            foreach (var name in new[] { ObjectKinds.MainName("w3b"), ObjectKinds.SkinName("w3b") })
            {
                var table = ObjectTable.Read(Path.Combine(project.MapFolder, name), "destructable");
                foreach (var entry in table.Originals.Concat(table.News))
                {
                    if (!types.TryGetValue(entry.Id, out var type))
                    {
                        type = new OwnType(entry.Id, entry.OldId, false, null, null);
                        types[entry.Id] = type;
                    }

                    foreach (var modification in entry.Modifications)
                    {
                        if (modification.Field == ModelField || modification.Field == RotationField)
                        {
                            type.Reshaped = true;
                        }
                        else if (modification.Field == WalkableField)
                        {
                            type.Walkable = Convert.ToInt32(modification.Value, CultureInfo.InvariantCulture) != 0;
                        }
                    }
                }
            }
        }

        // The type as the warning names it: its id and variation, and for the map's own its class, its base and whether it changes the deck.
        private static string Described(string id, string variation, OwnType? mine)
        {
            var text = id + " variation " + variation;
            if (mine == null)
            {
                return text;
            }

            var about = new List<string>();
            if (mine.ClassName != null)
            {
                about.Add(mine.ClassName);
            }

            if (mine.Base != id)
            {
                about.Add("based on " + mine.Base);
            }

            if (mine.Reshaped)
            {
                about.Add("its model or fixed rotation changed");
            }

            return about.Count == 0 ? text : text + " (" + string.Join(", ", about) + ")";
        }

        private static string Number(double value)
        {
            return value.ToString("0.###", CultureInfo.InvariantCulture);
        }

        // The text as the inside of a C# string literal: its line ends escaped. The text holds no quote or backslash.
        private static string Escaped(string text)
        {
            return text.Replace("\n", "\\n", StringComparison.Ordinal);
        }

        /// <summary>A destructable type of the map's own, declared in code or in its object files, as the bridges see it.</summary>
        public sealed class OwnType
        {
            /// <summary>Makes one.</summary>
            /// <param name="id">The type's id.</param>
            /// <param name="baseType">The id of the standard type it is based on; its own id when it changes a standard type.</param>
            /// <param name="reshaped">Whether it changes the model or the fixed rotation, which change the deck.</param>
            /// <param name="walkable">Whether it sets itself walkable or not; null when it keeps its base's.</param>
            /// <param name="className">The class that declares it in code; null for one from the object files.</param>
            public OwnType(string id, string baseType, bool reshaped, bool? walkable, string? className)
            {
                Id = id;
                Base = baseType;
                Reshaped = reshaped;
                Walkable = walkable;
                ClassName = className;
            }

            /// <summary>The type's id.</summary>
            public string Id { get; }

            /// <summary>The id of the standard type it is based on.</summary>
            public string Base { get; }

            /// <summary>Whether it changes the model or the fixed rotation.</summary>
            public bool Reshaped { get; internal set; }

            /// <summary>Whether it sets itself walkable or not; null when it keeps its base's.</summary>
            public bool? Walkable { get; internal set; }

            /// <summary>The class that declares it in code; null for one from the object files.</summary>
            public string? ClassName { get; }
        }
    }
}
