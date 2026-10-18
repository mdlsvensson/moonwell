using System.Globalization;

namespace Moonwell.Build
{
    // The values of one used class's fields as Lua literals: a declared class's changed values, else its standard root's; a
    // standard class's own. Lists and references are the objects' ids as numbers.
    internal sealed class FieldValues
    {
        private readonly UsedClass used;
        private readonly string root;
        private readonly IReadOnlyDictionary<string, object?> changed;
        private readonly IReadOnlyDictionary<string, string> ids;
        private readonly StandardData standard;
        private readonly string? set;
        private readonly Dictionary<string, StandardData.ObjectField> fields = new Dictionary<string, StandardData.ObjectField>(StringComparer.Ordinal);

        /// <param name="used">The class.</param>
        /// <param name="root">The standard object whose values the class has where it changed none.</param>
        /// <param name="changed">The values the class changed, as <see cref="DeclaredObject.Changed"/>; empty for a standard class.</param>
        /// <param name="ids">The object id of each class a changed list or reference may name.</param>
        /// <param name="standard">The standard data.</param>
        /// <param name="set">The balance set: null for the base tables.</param>
        public FieldValues(UsedClass used, string root, IReadOnlyDictionary<string, object?> changed, IReadOnlyDictionary<string, string> ids, StandardData standard, string? set)
        {
            this.used = used;
            this.root = root;
            this.changed = changed;
            this.ids = ids;
            this.standard = standard;
            this.set = set;
            foreach (var field in standard.Fields(used.Kind))
            {
                fields[field.Id] = field;
            }
        }

        /// <summary>The field's value as a Lua literal: a value, a table of the levels' values, a table of ids, or an id (0 for none).</summary>
        public string Literal(UsedField field)
        {
            var type = fields.TryGetValue(field.FieldId, out var known) ? known.Type : "string";
            switch (field.Access)
            {
                case "levels":
                    return Levels(field.FieldId, type);
                case "list":
                    return "{" + string.Join(", ", Entries(field.FieldId)) + "}";
                case "ref":
                    var entries = Entries(field.FieldId);
                    return entries.Count > 0 ? entries[0] : "0";
                default:
                    return Scalar(type, Raw(field.FieldId, 0));
            }
        }

        private string Levels(string field, string type)
        {
            var count = LevelCount();
            var prefix = field + ":";
            foreach (var key in changed.Keys)
            {
                if (key.StartsWith(prefix, StringComparison.Ordinal) && int.TryParse(key.Substring(prefix.Length), NumberStyles.Integer, CultureInfo.InvariantCulture, out var level))
                {
                    count = Math.Max(count, level);
                }
            }

            var values = new List<string>();
            for (var level = 1; level <= count; level++)
            {
                values.Add(Scalar(type, Raw(field, level)));
            }

            return "{" + string.Join(", ", values) + "}";
        }

        // The ids of a list or a reference; a field with levels has one list for every level, level 1's.
        private List<string> Entries(string field)
        {
            var value = changed.TryGetValue(field, out var whole) ? whole : changed.TryGetValue(field + ":1", out var first) ? first : null;
            IEnumerable<string> names;
            if (value is IReadOnlyList<string> list)
            {
                names = list;
            }
            else if (value is string one)
            {
                names = new[] { one };
            }
            else
            {
                var text = standard.Value(root, field, 0, set) ?? standard.Value(root, field, 1, set);
                names = string.IsNullOrEmpty(text) ? Array.Empty<string>() : text.Split(',').Select(id => id.Trim()).Where(id => id.Length == 4);
            }

            var entries = new List<string>();
            foreach (var name in names)
            {
                entries.Add(ObjectLua.FourCC(IdOf(name, field)).ToString(CultureInfo.InvariantCulture));
            }

            return entries;
        }

        private string IdOf(string name, string field)
        {
            if (ids.TryGetValue(name, out var id))
            {
                return id;
            }

            if (name.Length == 4 && !name.Contains('.'))
            {
                return name;
            }

            throw new BuildException($"{used.FullName}: {field} names {name}, which no object declares");
        }

        // A changed value as it was put, else the standard object's text; null for neither.
        private object? Raw(string field, int level)
        {
            var key = level > 0 ? field + ":" + level.ToString(CultureInfo.InvariantCulture) : field;
            return changed.TryGetValue(key, out var value) ? value : standard.Value(root, field, level, set);
        }

        private int LevelCount()
        {
            var field = ChangedFields.FieldOfLevelCount(used.Kind);
            if (field.Length == 0)
            {
                return 0;
            }

            if (changed.TryGetValue(field, out var put) && put is int count)
            {
                return count;
            }

            return int.TryParse(standard.Value(root, field, 0, set), NumberStyles.Integer, CultureInfo.InvariantCulture, out var standardCount) ? standardCount : 0;
        }

        // A value of a field's type: a put value is of the type; a standard text is read as the type, its default when it has
        // none. A string with no text is the empty text, not nil: Lua's .. fails on nil, where C# adds nothing.
        internal static string Scalar(string type, object? value)
        {
            switch (value)
            {
                case int number:
                    return number.ToString(CultureInfo.InvariantCulture);
                case float real:
                    return Real(real);
                case bool flag:
                    return flag ? "true" : "false";
            }

            var text = (string?)value;
            switch (type)
            {
                case "int":
                    if (int.TryParse(text, NumberStyles.Integer, CultureInfo.InvariantCulture, out var whole))
                    {
                        return whole.ToString(CultureInfo.InvariantCulture);
                    }

                    return float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var truncated) ? ((int)truncated).ToString(CultureInfo.InvariantCulture) : "0";
                case "float":
                    return float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var parsed) ? Real(parsed) : "0";
                case "bool":
                    return text == "1" ? "true" : "false";
                default:
                    return ObjectLua.Quote(text ?? string.Empty);
            }
        }

        private static string Real(float value)
        {
            if (float.IsNaN(value))
            {
                return "(0/0)";
            }

            if (float.IsInfinity(value))
            {
                return value > 0 ? "math.huge" : "-math.huge";
            }

            return value.ToString("R", CultureInfo.InvariantCulture);
        }
    }
}
