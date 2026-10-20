namespace Moonwell.Build
{
    /// <summary>Checks the object id each declared class gives itself in <c>[ObjectId]</c>, and its buff's carrier id; the build assigns none.</summary>
    public static class ObjectIds
    {
        private const string CarrierSuffix = "#carrier";
        private const string Digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ";
        private const int CounterLimit = 36 * 36 * 36;

        /// <summary>
        /// The object ids of a build, as the classes give them: a <c>[Replaces]</c> class has its standard object's id, every other
        /// declared class the id of its <c>[ObjectId]</c>, and a buff also the carrier ability's id its <c>[ObjectId]</c> gives.
        /// </summary>
        /// <param name="objects">The declared objects.</param>
        /// <param name="standardIds">The ids of the standard objects; no class may have one, and no suggested id is one.</param>
        /// <param name="sourceMapIds">The ids the source map's object files define; no suggested id is one of them.</param>
        /// <returns>The id by class name, and by <c>&lt;class&gt;#carrier</c> for the carrier ability of a buff.</returns>
        /// <exception cref="BuildException">
        /// A class has no id (the message names a free id for each such class), an id is not valid, or two classes have one id.
        /// </exception>
        public static IReadOnlyDictionary<string, string> Assign(
            IReadOnlyList<DeclaredObject> objects,
            IReadOnlySet<string> standardIds,
            IReadOnlySet<string> sourceMapIds)
        {
            var sorted = objects.OrderBy(declared => declared.ClassName, StringComparer.Ordinal).ToList();
            var claims = new Dictionary<string, Dictionary<string, string>>(StringComparer.Ordinal);
            foreach (var space in new[] { "unit", "item", "destructable", "doodad", "ability", "buff", "upgrade" })
            {
                claims[space] = new Dictionary<string, string>(StringComparer.Ordinal);
            }

            var ids = new SortedDictionary<string, string>(StringComparer.Ordinal);
            var missing = new List<DeclaredObject>();
            var missingCarrier = new List<DeclaredObject>();
            foreach (var declared in sorted)
            {
                if (declared.CarrierId != null && declared.Kind != "buff")
                {
                    throw new BuildException($"{declared.ClassName} is not a buff, so its [ObjectId] cannot give a carrier");
                }

                if (declared.Replaces)
                {
                    if (declared.PinnedId != null)
                    {
                        throw new BuildException($"{declared.ClassName} replaces {declared.StandardRoot} and cannot have an id of its own");
                    }

                    ids[declared.ClassName] = declared.StandardRoot;
                    continue;
                }

                if (declared.PinnedId == null)
                {
                    missing.Add(declared);
                    continue;
                }

                Check(declared.ClassName, declared.Kind, declared.PinnedId, standardIds);
                Claim(claims[SpaceOf(declared.Kind)], declared.PinnedId, declared.ClassName);
                ids[declared.ClassName] = declared.PinnedId;
                if (declared.Kind == "buff")
                {
                    if (declared.CarrierId == null)
                    {
                        missingCarrier.Add(declared);
                        continue;
                    }

                    var key = declared.ClassName + CarrierSuffix;
                    Check(key, "ability", declared.CarrierId, standardIds);
                    Claim(claims["ability"], declared.CarrierId, key);
                    ids[key] = declared.CarrierId;
                }
            }

            if (missing.Count > 0 || missingCarrier.Count > 0)
            {
                throw new BuildException(Suggestions(missing, missingCarrier, claims, standardIds, sourceMapIds));
            }

            return ids;
        }

        // One line per class with no id (a buff also with no carrier), then one per buff with no carrier, each naming free ids.
        private static string Suggestions(
            List<DeclaredObject> missing,
            List<DeclaredObject> missingCarrier,
            Dictionary<string, Dictionary<string, string>> claims,
            IReadOnlySet<string> standardIds,
            IReadOnlySet<string> sourceMapIds)
        {
            var lines = new List<string>();
            foreach (var declared in missing)
            {
                var id = Free(LetterOf(declared.Kind), claims[SpaceOf(declared.Kind)], standardIds, sourceMapIds, declared.ClassName);
                if (declared.Kind == "buff")
                {
                    var carrier = Free('A', claims["ability"], standardIds, sourceMapIds, declared.ClassName + CarrierSuffix);
                    lines.Add($"{declared.ClassName} has no object id; add [ObjectId(\"{id}\", Carrier = \"{carrier}\")] to it");
                }
                else
                {
                    lines.Add($"{declared.ClassName} has no object id; add [ObjectId(\"{id}\")] to it");
                }
            }

            foreach (var declared in missingCarrier)
            {
                var carrier = Free('A', claims["ability"], standardIds, sourceMapIds, declared.ClassName + CarrierSuffix);
                lines.Add($"{declared.ClassName} has no carrier id; add Carrier = \"{carrier}\" to its [ObjectId]");
            }

            return string.Join("\n", lines);
        }

        private static void Check(string owner, string kind, string id, IReadOnlySet<string> standardIds)
        {
            if (id.Length != 4)
            {
                throw new BuildException($"{owner} has the id \"{id}\", which is not four characters");
            }

            if (standardIds.Contains(id))
            {
                throw new BuildException($"{owner} has the id {id}, which a standard object has");
            }

            // The game makes a unit whose id starts with an upper-case letter a hero, and only such a unit.
            if (kind == "hero" && !char.IsAsciiLetterUpper(id[0]))
            {
                throw new BuildException($"{owner} is a hero, so its id {id} must start with an upper-case letter");
            }

            if (kind == "unit" && char.IsAsciiLetterUpper(id[0]))
            {
                throw new BuildException($"{owner} is a unit, so its id {id} must not start with an upper-case letter: the game makes such a unit a hero");
            }
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

        // The first id of the letter that nothing has, claimed for the owner so that the next suggestion differs.
        private static string Free(
            char letter,
            Dictionary<string, string> claimed,
            IReadOnlySet<string> standardIds,
            IReadOnlySet<string> sourceMapIds,
            string owner)
        {
            for (var count = 1; count < CounterLimit; count++)
            {
                var id = new string(new[] { letter, Digits[count / 1296], Digits[count / 36 % 36], Digits[count % 36] });
                if (standardIds.Contains(id) || sourceMapIds.Contains(id) || claimed.ContainsKey(id))
                {
                    continue;
                }

                claimed[id] = owner;
                return id;
            }

            throw new BuildException($"no free id starting with {letter} is left for {owner}");
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
    }
}
