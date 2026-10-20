namespace Moonwell.Build
{
    // The ability that puts a buff's icon on a unit: the game shows a buff only while an ability applies it, so each buff
    // with an object gets an ability that applies it to the unit itself.
    internal static class Carriers
    {
        private const string CarrierSuffix = "#carrier";
        private const string BaseAbility = "Aasl";

        /// <summary>The carrier ability of each declared buff, as an object to write; the buff itself is written like any declared object.</summary>
        /// <param name="objects">The declared objects.</param>
        /// <param name="ids">The object ids; a carrier is keyed <c>&lt;class&gt;#carrier</c>.</param>
        /// <returns>The carriers, in the order of their buffs.</returns>
        public static IReadOnlyList<DeclaredObject> Of(IReadOnlyList<DeclaredObject> objects, IReadOnlyDictionary<string, string> ids)
        {
            var carriers = new List<DeclaredObject>();
            foreach (var buff in objects)
            {
                if (buff.Kind != "buff")
                {
                    continue;
                }

                var name = buff.ClassName + CarrierSuffix;
                if (!ids.ContainsKey(name))
                {
                    throw new BuildException($"{buff.ClassName} has no carrier ability id");
                }

                var changed = new SortedDictionary<string, object?>(StringComparer.Ordinal)
                {
                    ["abuf:1"] = new[] { buff.ClassName },
                    ["atar:1"] = "self",
                    ["aare:1"] = 1f,
                };
                carriers.Add(new DeclaredObject(name, "ability", BaseAbility, false, null, null, changed, new[] { buff.ClassName }));
            }

            return carriers;
        }
    }
}
