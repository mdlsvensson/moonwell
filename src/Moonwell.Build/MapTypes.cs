using System.Globalization;
using War3Net.Build.Script;

namespace Moonwell.Build
{
    /// <summary>The unit, hero and item types the source map's object files make or change in World Editor.</summary>
    internal static class MapTypes
    {
        private const string TriggerStringPrefix = "TRIGSTR_";

        /// <summary>
        /// Every object of the source map's unit and item files, main and skin: a changed standard object (the original table)
        /// with its own id as its root, a new one with its base as its root, each with the values its files change. A unit whose
        /// id starts with a capital letter is a hero, as the generator has it. A text World Editor keeps in the trigger strings
        /// (<c>TRIGSTR_012</c>) is the string's text, as the game reads it.
        /// </summary>
        /// <param name="mapFolder">The map folder, which holds the object files.</param>
        /// <param name="strings">The map's trigger strings (<c>war3map.wts</c>); null when it has none.</param>
        /// <param name="standard">The standard data, whose fields give each stored value its type.</param>
        /// <returns>The types, in the files' order; empty when the map has no unit or item file.</returns>
        /// <exception cref="BuildException">An object file cannot be read.</exception>
        public static IReadOnlyList<MapType> Read(string mapFolder, TriggerStrings? strings, StandardData standard)
        {
            var texts = new Dictionary<uint, string>();
            foreach (var one in strings?.Strings ?? new List<TriggerString>())
            {
                texts[one.Key] = one.Value ?? string.Empty;
            }

            var types = new List<MapType>();
            foreach (var (kind, extension) in ObjectKinds.All.Where(one => one.Kind is "unit" or "item"))
            {
                var fields = new Dictionary<string, StandardData.ObjectField>(StringComparer.Ordinal);
                foreach (var field in standard.Fields(kind))
                {
                    fields[field.Id] = field;
                }

                // An object is in the main file and in the skin file, each with the fields it keeps there.
                var objects = new Dictionary<string, (string Root, SortedDictionary<string, object?> Changed)>(StringComparer.Ordinal);
                var order = new List<string>();
                foreach (var name in new[] { ObjectKinds.MainName(extension), ObjectKinds.SkinName(extension) })
                {
                    var table = ObjectTable.Read(Path.Combine(mapFolder, name), kind);
                    foreach (var entry in table.Originals.Concat(table.News))
                    {
                        if (!objects.TryGetValue(entry.Id, out var one))
                        {
                            objects[entry.Id] = one = (entry.OldId, new SortedDictionary<string, object?>(StringComparer.Ordinal));
                            order.Add(entry.Id);
                        }

                        // A unit's or an item's fields have no levels: the files hold level 0 for each (ObjectTable).
                        foreach (var modification in entry.Modifications)
                        {
                            one.Changed[modification.Field] = ValueOf(fields.GetValueOrDefault(modification.Field), modification.Value, texts);
                        }
                    }
                }

                foreach (var id in order)
                {
                    var kindOf = kind == "unit" && char.IsUpper(id[0]) ? "hero" : kind;
                    types.Add(new MapType(id, kindOf, objects[id].Root, objects[id].Changed));
                }
            }

            return types;
        }

        // A stored value as a declared one is kept: a flag stored as a number is a bool, a whole number of a real field a float,
        // a list's text its ids, an empty reference none; a choice stays its number or its game text, which both read.
        private static object? ValueOf(StandardData.ObjectField? field, object value, IReadOnlyDictionary<uint, string> texts)
        {
            if (value is string text)
            {
                value = TextOf(text, texts);
            }

            if (field == null || field.Choice != null)
            {
                return value;
            }

            switch (field.Type)
            {
                case "bool" when value is int number:
                    return number != 0;
                case "bool" when value is float real:
                    return real != 0f;
                case "int" when value is float real:
                    return (int)real;
                case "float" when value is int number:
                    return (float)number;
                case "objectList" when value is string list:
                    return list.Split(',').Select(id => id.Trim()).Where(id => id.Length == 4).ToList();
                case "objectRef" when value is string id && id.Length == 0:
                    return Array.Empty<string>();
                default:
                    return value;
            }
        }

        // A trigger string's text; a reference to no string stays as it is.
        private static string TextOf(string text, IReadOnlyDictionary<uint, string> texts)
        {
            if (text.StartsWith(TriggerStringPrefix, StringComparison.Ordinal)
                && uint.TryParse(text.Substring(TriggerStringPrefix.Length), NumberStyles.None, CultureInfo.InvariantCulture, out var key)
                && texts.TryGetValue(key, out var found))
            {
                return found;
            }

            return text;
        }
    }
}
