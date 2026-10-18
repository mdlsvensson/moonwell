using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;

namespace Moonwell.Build
{
    /// <summary>Gives each declared object its four-character object id, kept stable across builds in the project's <c>ids.json</c>.</summary>
    public static class ObjectIds
    {
        private const string FileName = "ids.json";
        private const string CarrierSuffix = "#carrier";
        private const string Digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";
        private const int CounterLimit = 36 * 36 * 36;

        private static readonly string[] AllSpaces = { "unit", "item", "destructable", "doodad", "ability", "buff", "upgrade" };

        private static readonly JsonSerializerOptions TextOptions = new JsonSerializerOptions
        {
            Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
        };

        /// <summary>
        /// Assigns the object ids of a build: keeps the id of each class <c>ids.json</c> already holds, applies <c>[ObjectId]</c> pins,
        /// gives new classes the next free id of their kind, gives each buff a carrier ability, reports renamed classes and rewrites <c>ids.json</c>.
        /// </summary>
        /// <param name="project">The map project; <c>ids.json</c> is in its folder.</param>
        /// <param name="objects">The declared objects.</param>
        /// <param name="standardIds">The ids of the standard objects; no new id is one of them.</param>
        /// <param name="sourceMapIds">The ids the source map's object files define; no new id is one of them.</param>
        /// <param name="placedIds">The ids of the objects the source map places (units, items, destructables and doodads).</param>
        /// <param name="log">The build log; warnings start with <c>warning: </c>.</param>
        /// <returns>The id by class name, and by <c>&lt;class&gt;#carrier</c> for the carrier ability of a buff.</returns>
        /// <exception cref="BuildException"><c>ids.json</c> is malformed, a pin is not valid or clashes, or a renamed class's id is placed in the map.</exception>
        public static IReadOnlyDictionary<string, string> Assign(
            MapProject project,
            IReadOnlyList<DeclaredObject> objects,
            IReadOnlySet<string> standardIds,
            IReadOnlySet<string> sourceMapIds,
            IReadOnlySet<string> placedIds,
            TextWriter log)
        {
            var path = Path.Combine(project.Folder, FileName);
            var old = Read(path);
            var sorted = objects.OrderBy(declared => declared.ClassName, StringComparer.Ordinal).ToList();
            var declaredKinds = new Dictionary<string, string>(StringComparer.Ordinal);
            foreach (var declared in sorted)
            {
                declaredKinds[declared.ClassName] = declared.Kind;
            }

            var claims = new Dictionary<string, Dictionary<string, string>>(StringComparer.Ordinal);
            var reserved = new Dictionary<string, HashSet<string>>(StringComparer.Ordinal);
            foreach (var space in AllSpaces)
            {
                claims[space] = new Dictionary<string, string>(StringComparer.Ordinal);
                reserved[space] = new HashSet<string>(StringComparer.Ordinal);
            }

            foreach (var entry in old)
            {
                foreach (var space in SpacesOf(entry.Key, entry.Value, declaredKinds))
                {
                    reserved[space].Add(entry.Value);
                }
            }

            var ids = new SortedDictionary<string, string>(StringComparer.Ordinal);
            var replacing = new HashSet<string>(StringComparer.Ordinal);
            var pending = new List<DeclaredObject>();
            foreach (var declared in sorted)
            {
                if (declared.Replaces)
                {
                    if (declared.PinnedId != null)
                    {
                        throw new BuildException($"{declared.ClassName} replaces {declared.StandardRoot} and cannot pin an id");
                    }

                    ids[declared.ClassName] = declared.StandardRoot;
                    replacing.Add(declared.ClassName);
                    continue;
                }

                string id;
                if (declared.PinnedId != null)
                {
                    if (declared.PinnedId.Length != 4)
                    {
                        throw new BuildException($"{declared.ClassName} pins the id \"{declared.PinnedId}\", which is not four characters");
                    }

                    if (standardIds.Contains(declared.PinnedId))
                    {
                        throw new BuildException($"{declared.ClassName} pins the id {declared.PinnedId}, which a standard object has");
                    }

                    id = declared.PinnedId;
                }
                else if (old.TryGetValue(declared.ClassName, out var kept))
                {
                    id = kept;
                }
                else
                {
                    pending.Add(declared);
                    continue;
                }

                Claim(claims[SpaceOf(declared.Kind)], id, declared.ClassName);
                ids[declared.ClassName] = id;
            }

            var pendingCarriers = new List<string>();
            foreach (var declared in sorted)
            {
                if (declared.Kind != "buff")
                {
                    continue;
                }

                var key = declared.ClassName + CarrierSuffix;
                if (old.TryGetValue(key, out var kept))
                {
                    Claim(claims["ability"], kept, key);
                    ids[key] = kept;
                }
                else
                {
                    pendingCarriers.Add(key);
                }
            }

            ReportRenames(old, declaredKinds, pending, claims, placedIds, log);

            var counters = new Dictionary<string, int>(StringComparer.Ordinal);
            foreach (var declared in pending)
            {
                var space = SpaceOf(declared.Kind);
                var id = Next(counters, LetterOf(declared.Kind), space, standardIds, sourceMapIds, reserved[space], claims[space], declared.ClassName);
                Claim(claims[space], id, declared.ClassName);
                ids[declared.ClassName] = id;
            }

            foreach (var key in pendingCarriers)
            {
                var id = Next(counters, 'A', "ability", standardIds, sourceMapIds, reserved["ability"], claims["ability"], key);
                Claim(claims["ability"], id, key);
                ids[key] = id;
            }

            Write(path, ids, replacing);
            return ids;
        }

        private static void ReportRenames(
            Dictionary<string, string> old,
            Dictionary<string, string> declaredKinds,
            List<DeclaredObject> pending,
            Dictionary<string, Dictionary<string, string>> claims,
            IReadOnlySet<string> placedIds,
            TextWriter log)
        {
            var suggested = new HashSet<string>(StringComparer.Ordinal);
            foreach (var entry in old.OrderBy(pair => pair.Key, StringComparer.Ordinal))
            {
                if (entry.Key.EndsWith(CarrierSuffix, StringComparison.Ordinal) || declaredKinds.ContainsKey(entry.Key))
                {
                    continue;
                }

                var spaces = SpacesOfId(entry.Value);
                var pinned = false;
                foreach (var space in spaces)
                {
                    pinned |= claims[space].ContainsKey(entry.Value);
                }

                if (pinned)
                {
                    continue;
                }

                var candidates = pending.Where(declared => spaces.Contains(SpaceOf(declared.Kind))).ToList();
                if (candidates.Count == 0)
                {
                    continue;
                }

                var target = candidates.FirstOrDefault(declared => !suggested.Contains(declared.ClassName)) ?? candidates[0];
                suggested.Add(target.ClassName);
                var advice = $"if it was renamed to {target.ClassName}, add [ObjectId(\"{entry.Value}\")] to {target.ClassName}";
                if (placedIds.Contains(entry.Value))
                {
                    throw new BuildException($"{entry.Key} was {entry.Value}, which the map places, and no class declares it now; {advice}");
                }

                log.WriteLine($"warning: {entry.Key} was {entry.Value}, and no class declares it now; {advice}");
            }
        }

        private static Dictionary<string, string> Read(string path)
        {
            var entries = new Dictionary<string, string>(StringComparer.Ordinal);
            if (!File.Exists(path))
            {
                return entries;
            }

            var problem = path + " is not a JSON object from class name to id: ";
            try
            {
                using var document = JsonDocument.Parse(File.ReadAllBytes(path));
                if (document.RootElement.ValueKind != JsonValueKind.Object)
                {
                    throw new BuildException(problem + "it holds a " + document.RootElement.ValueKind.ToString().ToLowerInvariant());
                }

                foreach (var property in document.RootElement.EnumerateObject())
                {
                    if (property.Value.ValueKind != JsonValueKind.String || property.Value.GetString()!.Length != 4)
                    {
                        throw new BuildException(problem + $"the value of \"{property.Name}\" is not a four-character id");
                    }

                    if (!entries.TryAdd(property.Name, property.Value.GetString()!))
                    {
                        throw new BuildException(problem + $"\"{property.Name}\" is there twice");
                    }
                }
            }
            catch (JsonException exception)
            {
                throw new BuildException(problem + exception.Message);
            }

            return entries;
        }

        private static void Write(string path, SortedDictionary<string, string> ids, HashSet<string> replacing)
        {
            var text = new StringBuilder("{\n");
            var first = true;
            foreach (var entry in ids)
            {
                if (replacing.Contains(entry.Key))
                {
                    continue;
                }

                if (!first)
                {
                    text.Append(",\n");
                }

                first = false;
                text.Append("  ").Append(JsonSerializer.Serialize(entry.Key, TextOptions))
                    .Append(": ").Append(JsonSerializer.Serialize(entry.Value, TextOptions));
            }

            if (!first)
            {
                text.Append('\n');
            }

            text.Append("}\n");
            var bytes = new UTF8Encoding(false).GetBytes(text.ToString());
            if (first && !File.Exists(path))
            {
                return;
            }

            if (File.Exists(path) && File.ReadAllBytes(path).AsSpan().SequenceEqual(bytes))
            {
                return;
            }

            File.WriteAllBytes(path, bytes);
        }

        private static void Claim(Dictionary<string, string> claimed, string id, string owner)
        {
            if (claimed.TryGetValue(id, out var other))
            {
                var pair = new[] { other, owner };
                Array.Sort(pair, StringComparer.Ordinal);
                throw new BuildException($"{pair[0]} and {pair[1]} both have the id {id}");
            }

            claimed[id] = owner;
        }

        private static string Next(
            Dictionary<string, int> counters,
            char letter,
            string space,
            IReadOnlySet<string> standardIds,
            IReadOnlySet<string> sourceMapIds,
            HashSet<string> reserved,
            Dictionary<string, string> claimed,
            string owner)
        {
            var counterKey = letter + space;
            counters.TryGetValue(counterKey, out var count);
            while (true)
            {
                count++;
                if (count >= CounterLimit)
                {
                    throw new BuildException($"no free id starting with {letter} is left for {owner}");
                }

                var id = new string(new[] { letter, Digits[count / 1296], Digits[count / 36 % 36], Digits[count % 36] });
                if (standardIds.Contains(id) || sourceMapIds.Contains(id) || reserved.Contains(id) || claimed.ContainsKey(id))
                {
                    continue;
                }

                counters[counterKey] = count;
                return id;
            }
        }

        private static string SpaceOf(string kind)
        {
            switch (kind)
            {
                case "unit":
                case "hero":
                    return "unit";
                case "item":
                case "destructable":
                case "doodad":
                case "ability":
                case "buff":
                case "upgrade":
                    return kind;
                default:
                    throw new BuildException($"unknown object kind {kind}");
            }
        }

        private static char LetterOf(string kind)
        {
            switch (kind)
            {
                case "hero":
                    return 'H';
                case "unit":
                    return 'h';
                case "item":
                    return 'I';
                case "ability":
                    return 'A';
                case "buff":
                case "destructable":
                    return 'B';
                case "doodad":
                    return 'D';
                case "upgrade":
                    return 'R';
                default:
                    throw new BuildException($"unknown object kind {kind}");
            }
        }

        // The id spaces an ids.json entry can be in: its class's when the class is still declared, a carrier's ability space, else the id's first letter tells.
        private static IReadOnlyList<string> SpacesOf(string key, string id, Dictionary<string, string> declaredKinds)
        {
            if (key.EndsWith(CarrierSuffix, StringComparison.Ordinal))
            {
                return new[] { "ability" };
            }

            return declaredKinds.TryGetValue(key, out var kind) ? new[] { SpaceOf(kind) } : SpacesOfId(id);
        }

        private static IReadOnlyList<string> SpacesOfId(string id)
        {
            switch (id[0])
            {
                case 'H':
                case 'h':
                    return new[] { "unit" };
                case 'I':
                    return new[] { "item" };
                case 'A':
                    return new[] { "ability" };
                case 'B':
                    return new[] { "buff", "destructable" };
                case 'D':
                    return new[] { "doodad" };
                case 'R':
                    return new[] { "upgrade" };
                default:
                    return AllSpaces;
            }
        }
    }
}
