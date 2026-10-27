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
    /// base's shape, unless it changes the model or the fixed rotation, which would change the deck. A placed walkable type
    /// with no shape is a warning: the package does not see it as ground.
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
        /// <param name="log">Where a placed walkable type with no deck shape is warned of.</param>
        /// <param name="decks">The shapes of the types placed, in the order the bridges number them: <c>cell columns rows originColumn originRow</c>, then the rows.</param>
        /// <returns>The text.</returns>
        public static string Encode(MapProject project, MapDoodads? doodads, TextWriter log, out string decks)
        {
            var shapes = Shapes(out var unmeasured);
            var news = new Dictionary<string, (string Base, HashSet<string> Changed, bool? Walkable)>(StringComparer.Ordinal);
            ReadObjects(project, news);

            var bridges = new StringBuilder();
            var used = new List<string>();
            var warned = new HashSet<string>(StringComparer.Ordinal);
            foreach (var doodad in doodads?.Doodads ?? new List<DoodadData>())
            {
                var type = ObjectTable.TextOf(doodad.TypeId);
                var baseType = type;
                var changed = new HashSet<string>(StringComparer.Ordinal);
                bool? walkable = null;
                if (news.TryGetValue(type, out var own))
                {
                    baseType = own.Base;
                    changed = own.Changed;
                    walkable = own.Walkable;
                }

                var key = baseType + " " + doodad.Variation.ToString(CultureInfo.InvariantCulture);
                var standard = shapes.ContainsKey(key) || unmeasured.Contains(baseType) || shapes.Keys.Any(shape => shape.StartsWith(baseType + " ", StringComparison.Ordinal));
                if (!(walkable ?? standard))
                {
                    continue;
                }

                if (!shapes.ContainsKey(key) || changed.Contains(ModelField) || changed.Contains(RotationField))
                {
                    if (warned.Add(type))
                    {
                        log.WriteLine($"warning: war3map.doo places {Described(type, baseType, changed)}, a walkable destructable with no deck shape: the physics package does not see it as ground");
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
                bridges.Append(type)
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

        /// <summary>
        /// Writes <c>PhysicsData.g.cs</c> into the project's generated folder: <c>Moonwell.Generated.PhysicsData</c> with the bridges'
        /// text from <c>Bridges()</c> and the shapes' from <c>Decks()</c>, methods so that CSharp.lua writes each text once.
        /// </summary>
        /// <param name="project">The map project.</param>
        /// <param name="doodads">The map's placed doodads and destructables; null when it has no <c>war3map.doo</c>.</param>
        /// <param name="log">Where a placed walkable type with no deck shape is warned of.</param>
        /// <returns>The path of the written file.</returns>
        public static string Write(MapProject project, MapDoodads? doodads, TextWriter log)
        {
            var bridges = Encode(project, doodads, log, out var decks);
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
        // each one's base, the fields it changes, and whether it sets itself walkable.
        private static void ReadObjects(MapProject project, Dictionary<string, (string Base, HashSet<string> Changed, bool? Walkable)> types)
        {
            foreach (var name in new[] { ObjectKinds.MainName("w3b"), ObjectKinds.SkinName("w3b") })
            {
                var table = ObjectTable.Read(Path.Combine(project.MapFolder, name), "destructable");
                foreach (var entry in table.Originals.Concat(table.News))
                {
                    if (!types.TryGetValue(entry.Id, out var type))
                    {
                        type = (entry.OldId, new HashSet<string>(StringComparer.Ordinal), null);
                    }

                    foreach (var modification in entry.Modifications)
                    {
                        type.Changed.Add(modification.Field);
                        if (modification.Field == WalkableField)
                        {
                            type.Walkable = Convert.ToInt32(modification.Value, CultureInfo.InvariantCulture) != 0;
                        }
                    }

                    types[entry.Id] = type;
                }
            }
        }

        // The type as the warning names it: its id, its base when it is the map's own, and whether it changes the deck.
        private static string Described(string type, string baseType, HashSet<string> changed)
        {
            var reshaped = changed.Contains(ModelField) || changed.Contains(RotationField);
            if (type == baseType)
            {
                return reshaped ? type + " (its model or fixed rotation changed)" : type;
            }

            return $"{type} (based on {baseType}" + (reshaped ? ", its model or fixed rotation changed)" : ")");
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
    }
}
