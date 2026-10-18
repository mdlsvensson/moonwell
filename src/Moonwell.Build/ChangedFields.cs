using System.Globalization;
using System.Reflection;

namespace Moonwell.Build
{
    // Compares the values a declared class put on its fields with the standard object at the root of its base chain,
    // and keeps the ones that differ.
    internal sealed class ChangedFields
    {
        private const BindingFlags Internal = BindingFlags.Instance | BindingFlags.NonPublic;

        private readonly StandardData standard;
        private readonly string? set;
        private readonly MapProject project;
        private readonly IReadOnlyDictionary<string, string> classNames;

        /// <param name="standard">The standard values.</param>
        /// <param name="set">The tables the unset fields read (<see cref="GameData.ValueSet"/>); null for the base tables.</param>
        /// <param name="project">The map project; lines in messages are relative to its folder.</param>
        /// <param name="classNames">The full class name of each standard id that has a standard class.</param>
        public ChangedFields(StandardData standard, string? set, MapProject project, IReadOnlyDictionary<string, string> classNames)
        {
            this.standard = standard;
            this.set = set;
            this.project = project;
            this.classNames = classNames;
        }

        /// <summary>Gives the values that differ from the standard object, and adds the classes its lists and references hold to <paramref name="references"/>.</summary>
        /// <param name="className">The declared class, for messages.</param>
        /// <param name="kind">The object's kind.</param>
        /// <param name="root">The id of the standard object at the root of the class's base chain.</param>
        /// <param name="declared">The values the class put, by field id.</param>
        /// <param name="references">Where the classes of changed lists and references are added.</param>
        public SortedDictionary<string, object?> Compute(string className, string kind, string root, IReadOnlyDictionary<string, object?> declared, ISet<string> references)
        {
            var fields = new Dictionary<string, StandardData.ObjectField>(StringComparer.Ordinal);
            foreach (var field in standard.Fields(kind))
            {
                fields[field.Id] = field;
            }

            var levels = LevelCount(kind, root, declared);
            var changed = new SortedDictionary<string, object?>(StringComparer.Ordinal);

            // A whole field first; a level put by itself afterwards, so that it wins over the whole field's.
            foreach (var entry in declared)
            {
                if (!entry.Key.Contains(':') && entry.Value != null)
                {
                    Whole(className, fields[entry.Key], entry.Value, root, levels, changed, references);
                }
            }

            foreach (var entry in declared)
            {
                var colon = entry.Key.LastIndexOf(':');
                if (colon >= 0)
                {
                    var field = fields[entry.Key.Substring(0, colon)];
                    var level = int.Parse(entry.Key.Substring(colon + 1), CultureInfo.InvariantCulture);
                    if (Same(field.Type, entry.Value, standard.Value(root, field.Id, level, set)))
                    {
                        changed.Remove(entry.Key);
                    }
                    else
                    {
                        changed[entry.Key] = entry.Value;
                    }
                }
            }

            return changed;
        }

        // The field that holds the number of levels of an object of a kind; empty for a kind without levels.
        internal static string FieldOfLevelCount(string kind)
        {
            switch (kind)
            {
                case "ability":
                    return "alev";
                case "upgrade":
                    return "glvl";
                case "doodad":
                    return "dvar";
                default:
                    return string.Empty;
            }
        }

        // The number of levels the object has: what the class put, else the standard object's. Doodads count their variations.
        private int LevelCount(string kind, string root, IReadOnlyDictionary<string, object?> declared)
        {
            var field = FieldOfLevelCount(kind);
            if (field.Length == 0)
            {
                return 0;
            }

            if (declared.TryGetValue(field, out var put) && put is int count)
            {
                return count;
            }

            return int.TryParse(standard.Value(root, field, 0, set), NumberStyles.Integer, CultureInfo.InvariantCulture, out var standardCount) ? standardCount : 0;
        }

        private void Whole(string className, StandardData.ObjectField field, object value, string root, int levels, SortedDictionary<string, object?> changed, ISet<string> references)
        {
            var type = value.GetType();
            var generic = type.IsGenericType ? type.GetGenericTypeDefinition().FullName : null;
            switch (generic)
            {
                case "Moonwell.Objects.Levels`1":
                    WholeLevels(className, field, value, type, root, levels, changed);
                    break;
                case "Moonwell.Objects.ObjectList`1":
                    WholeList(
                        field,
                        (IReadOnlyList<object>)type.GetProperty("Entries", Internal)!.GetValue(value)!,
                        (IReadOnlyList<object>)type.GetProperty("Standard", Internal)!.GetValue(value)!,
                        root,
                        levels,
                        changed,
                        references);
                    break;
                case "Moonwell.Objects.ObjectRef`1":
                    WholeReference(field, (Type?)type.GetProperty("Type", Internal)!.GetValue(value), root, levels, changed, references);
                    break;
                default:
                    if (!Same(field.Type, value, standard.Value(root, field.Id, 0, set)))
                    {
                        changed[field.Id] = value;
                    }

                    break;
            }
        }

        // A field with a value for each level is written with every level when any one differs.
        private void WholeLevels(string className, StandardData.ObjectField field, object value, Type type, string root, int levels, SortedDictionary<string, object?> changed)
        {
            try
            {
                type.GetMethod("Resolve", Internal)!.Invoke(value, new object[] { levels });
            }
            catch (TargetInvocationException wrapped) when (wrapped.InnerException != null)
            {
                // A formula that throws or calls the game fails the way a constructor does.
                throw MapClasses.ConstructionError(wrapped.InnerException, className, project);
            }

            var values = (Array)type.GetProperty("Values", Internal)!.GetValue(value)!;
            var differs = false;
            for (var i = 0; i < values.Length; i++)
            {
                differs |= !Same(field.Type, values.GetValue(i), standard.Value(root, field.Id, i + 1, set));
            }

            if (differs)
            {
                for (var i = 0; i < values.Length; i++)
                {
                    changed[Key(field.Id, i + 1)] = values.GetValue(i);
                }
            }
        }

        // The list as the constructor left it. A list the constructor did not change is not written. A field with levels
        // has one list for every level, written under each level when it differs from the standard list of any level.
        private void WholeList(StandardData.ObjectField field, IReadOnlyList<object> entries, IReadOnlyList<object> started, string root, int levels, SortedDictionary<string, object?> changed, ISet<string> references)
        {
            var final = entries.Select(NameOf).ToArray();
            if (final.SequenceEqual(started.Select(NameOf)))
            {
                return;
            }

            var first = field.PerLevel ? 1 : 0;
            var last = field.PerLevel ? levels : 0;
            var differs = false;
            for (var level = first; level <= last; level++)
            {
                differs |= !final.SequenceEqual(NamesOf(standard.Value(root, field.Id, level, set)));
            }

            if (!differs)
            {
                return;
            }

            for (var level = first; level <= last; level++)
            {
                changed[field.PerLevel ? Key(field.Id, level) : field.Id] = final;
            }

            foreach (var entry in entries)
            {
                if (entry is Type type)
                {
                    references.Add(MapClasses.Describe(type));
                }
            }
        }

        private void WholeReference(StandardData.ObjectField field, Type? target, string root, int levels, SortedDictionary<string, object?> changed, ISet<string> references)
        {
            if (target == null)
            {
                return;
            }

            var name = MapClasses.Describe(target);
            var levelKeys = field.PerLevel ? levels : 0;
            var differs = false;
            for (var level = field.PerLevel ? 1 : 0; level <= levelKeys; level++)
            {
                var names = NamesOf(standard.Value(root, field.Id, level, set));
                differs |= names.Count != 1 || names[0] != name;
            }

            if (!differs)
            {
                return;
            }

            for (var level = field.PerLevel ? 1 : 0; level <= levelKeys; level++)
            {
                changed[field.PerLevel ? Key(field.Id, level) : field.Id] = name;
            }

            references.Add(name);
        }

        // The class names of a standard list's ids; an id with no standard class stays as it is.
        private List<string> NamesOf(string? text)
        {
            var names = new List<string>();
            if (!string.IsNullOrEmpty(text))
            {
                foreach (var id in text.Split(','))
                {
                    names.Add(classNames.TryGetValue(id, out var name) ? name : id);
                }
            }

            return names;
        }

        // A class's full name, or the id of a standard object that has no class.
        private static string NameOf(object entry)
        {
            return entry is Type type ? MapClasses.Describe(type) : (string)entry;
        }

        private static string Key(string field, int level)
        {
            return field + ":" + level.ToString(CultureInfo.InvariantCulture);
        }

        // Whether a value is the one the standard text says; a standard object with no value reads as the type's default.
        private static bool Same(string type, object? value, string? text)
        {
            switch (type)
            {
                case "int":
                    var number = (int)value!;
                    return text == null ? number == 0 : int.TryParse(text, NumberStyles.Integer, CultureInfo.InvariantCulture, out var standardNumber) && standardNumber == number;
                case "float":
                    var real = (float)value!;
                    return text == null ? real == 0f : float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var standardReal) && standardReal == real;
                case "bool":
                    return (bool)value! == (text == "1");
                default:
                    return ((string?)value ?? string.Empty) == (text ?? string.Empty);
            }
        }
    }
}
