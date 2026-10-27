using System.Diagnostics;
using System.Globalization;
using System.Reflection;

namespace Moonwell.Build
{
    // Compares the values a declared class put on its fields with the standard object at the root of its base chain,
    // and keeps the ones that differ.
    internal sealed class ChangedFields
    {
        private const BindingFlags Internal = BindingFlags.Instance | BindingFlags.NonPublic;
        private const string SharedFieldAttributeName = "Moonwell.Objects.SharedFieldAttribute";
        private const string BeyondLimitsAttributeName = "Moonwell.Objects.BeyondLimitsAttribute";

        private readonly StandardData standard;
        private readonly string? set;
        private readonly MapProject project;
        private readonly IReadOnlyDictionary<string, string> classNames;
        private readonly HeroBonus bonus;

        /// <param name="standard">The standard values.</param>
        /// <param name="set">The tables the unset fields read (<see cref="GameData.ValueSet"/>); null for the base tables.</param>
        /// <param name="project">The map project; lines in messages are relative to its folder.</param>
        /// <param name="classNames">The full class name of each standard id that has a standard class.</param>
        /// <param name="bonus">What a point of a hero's strength and intelligence adds, by the map's gameplay constants.</param>
        public ChangedFields(StandardData standard, string? set, MapProject project, IReadOnlyDictionary<string, string> classNames, HeroBonus bonus)
        {
            this.standard = standard;
            this.set = set;
            this.project = project;
            this.classNames = classNames;
            this.bonus = bonus;
        }

        /// <summary>
        /// A hero's puts with the totals its class put by their live names (<c>MaxLife = 800</c>) made base fields: the total less
        /// what the hero's attribute adds to it (strength to most life, intelligence to most and starting mana), the attribute the
        /// class leaves the hero with, so that the hero starts with the total. A field with no total put keeps its base, and follows
        /// the attribute.
        /// </summary>
        /// <param name="type">The declared class, whose chain carries the fields' rules.</param>
        /// <param name="root">The id of the standard object at the root of the class's base chain.</param>
        /// <param name="declared">The values the class put, by field id.</param>
        /// <param name="totals">The totals the class put, by field id.</param>
        /// <param name="putAt">Where each total was put, a <see cref="StackTrace"/>.</param>
        /// <exception cref="BuildException">A base is outside its field's limits; the message has the line the total was put on.</exception>
        public IReadOnlyDictionary<string, object?> WithBases(Type type, string root, IReadOnlyDictionary<string, object?> declared, IReadOnlyDictionary<string, object?> totals, IReadOnlyDictionary<string, object?> putAt)
        {
            if (totals.Count == 0)
            {
                return declared;
            }

            var merged = new Dictionary<string, object?>(declared, StringComparer.Ordinal);
            foreach (var entry in totals.OrderBy(entry => entry.Key, StringComparer.Ordinal))
            {
                var field = entry.Key;
                var total = (int)entry.Value!;
                var attribute = HeroBonus.AttributeOf(field)!;
                var points = declared.TryGetValue(attribute, out var put) && put is int number ? number : StandardWhole(root, attribute);
                var given = points * bonus.PerPoint(field);

                // The smallest base that the game's whole-number total (HeroBonus.Total) brings to the total put.
                var value = (int)Math.Ceiling(total - (double)given);
                CheckBase(type, field, total, value, points, given, putAt.GetValueOrDefault(field) as StackTrace);
                merged[field] = value;
            }

            return merged;
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
                    if (Same(field, entry.Value, standard.Value(root, field.Id, level, set)))
                    {
                        changed.Remove(entry.Key);
                    }
                    else
                    {
                        changed[entry.Key] = Plain(entry.Value);
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

        // A base outside its field's limits ([SharedField] on the kind base, lifted by [BeyondLimits] of its live member): the
        // total and what the attribute gives are named, with the totals the limits allow.
        private void CheckBase(Type type, string field, int total, int value, int points, float given, StackTrace? putAt)
        {
            var rule = type.GetCustomAttributesData().Concat(KindBaseAttributes(type))
                .FirstOrDefault(data => data.AttributeType.FullName == SharedFieldAttributeName && (string?)data.ConstructorArguments[0].Value == field);
            if (rule == null || Named<bool>(rule, "HasLimits") != true)
            {
                return;
            }

            var member = (string)rule.ConstructorArguments[1].Value!;
            if (Lifted(type, member))
            {
                return;
            }

            var min = Named<double>(rule, "Min") ?? double.NegativeInfinity;
            var max = Named<double>(rule, "Max") ?? double.PositiveInfinity;
            var attribute = HeroBonus.AttributeOf(field) == "ustr" ? "strength" : "intelligence";
            var shown = given.ToString("R", CultureInfo.InvariantCulture);
            string? problem = null;
            if (value < min)
            {
                var least = (int)Math.Floor(min - 1 + given) + 1;
                problem = $"its {points} {attribute} already give {shown} ({least} at least)";
            }
            else if (value > max)
            {
                var most = (int)Math.Floor(max + given);
                problem = $"its {points} {attribute} give {shown} ({most} at most)";
            }

            if (problem != null)
            {
                var place = putAt == null ? null : MapClasses.FirstLineInProject(putAt, project);
                throw new BuildException($"{(place == null ? string.Empty : place + ": ")}{RuleName(type)}.{member} is {total}; {problem}");
            }
        }

        // The attributes of the classes above the declared one, up to the kind base that carries the shared fields.
        private static IEnumerable<CustomAttributeData> KindBaseAttributes(Type type)
        {
            for (var parent = type.BaseType; parent != null; parent = parent.BaseType)
            {
                foreach (var data in parent.GetCustomAttributesData())
                {
                    yield return data;
                }
            }
        }

        // Whether [BeyondLimits] on the class or a base names the member.
        private static bool Lifted(Type type, string member)
        {
            for (var parent = type; parent != null; parent = parent.BaseType)
            {
                foreach (var data in parent.GetCustomAttributesData())
                {
                    if (data.AttributeType.FullName == BeyondLimitsAttributeName
                        && data.ConstructorArguments.Count == 1
                        && data.ConstructorArguments[0].Value is IReadOnlyCollection<CustomAttributeTypedArgument> names
                        && names.Any(name => (string?)name.Value == member))
                    {
                        return true;
                    }
                }
            }

            return false;
        }

        private static T? Named<T>(CustomAttributeData data, string name)
            where T : struct
        {
            foreach (var named in data.NamedArguments)
            {
                if (named.MemberName == name)
                {
                    return (T)named.TypedValue.Value!;
                }
            }

            return null;
        }

        // The class's name as its rules' errors name it, without its namespace (RuleChecks in Moonwell.Objects): Champion, Heroes.Champion for a nested class.
        private static string RuleName(Type type)
        {
            var name = type.Name;
            for (var outer = type.DeclaringType; outer != null; outer = outer.DeclaringType)
            {
                name = outer.Name + "." + name;
            }

            return name;
        }

        // A whole-number field of the standard object, as the build reads it; 0 when it has none.
        private int StandardWhole(string root, string field)
        {
            return int.TryParse(standard.Value(root, field, 0, set), NumberStyles.Integer, CultureInfo.InvariantCulture, out var number) ? number : 0;
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
                        (IReadOnlyList<object[]>)type.GetProperty("Changes", Internal)!.GetValue(value)!,
                        root,
                        levels,
                        changed,
                        references);
                    break;
                case "Moonwell.Objects.ObjectRef`1":
                    WholeReference(field, (Type?)type.GetProperty("Type", Internal)!.GetValue(value), root, levels, changed, references);
                    break;
                default:
                    var typed = InFieldsType(field, value);
                    if (!Same(field, typed, standard.Value(root, field.Id, 0, set)))
                    {
                        changed[field.Id] = Plain(typed);
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
                differs |= !Same(field, values.GetValue(i), standard.Value(root, field.Id, i + 1, set));
            }

            if (differs)
            {
                for (var i = 0; i < values.Length; i++)
                {
                    changed[Key(field.Id, i + 1)] = Plain(values.GetValue(i));
                }
            }
        }

        // The list as the constructor left it. A list the constructor did not change is not written. A field with levels has
        // a list for each level: what the constructor did to its list (added, removed, replaced, set) is done again to each
        // level's own standard list (decision 72), and every level is written when any one differs from its standard list.
        private void WholeList(StandardData.ObjectField field, IReadOnlyList<object> entries, IReadOnlyList<object> started, IReadOnlyList<object[]> changes, string root, int levels, SortedDictionary<string, object?> changed, ISet<string> references)
        {
            var classes = new HashSet<string>(classNames.Values, StringComparer.Ordinal);
            foreach (var entry in entries.Concat(changes.SelectMany(change => change.Skip(1))))
            {
                if (entry is Type type)
                {
                    classes.Add(MapClasses.Describe(type));
                }
            }

            if (!field.PerLevel)
            {
                var final = entries.Select(NameOf).ToArray();
                if (final.SequenceEqual(started.Select(NameOf)) || final.SequenceEqual(NamesOf(standard.Value(root, field.Id, 0, set))))
                {
                    return;
                }

                changed[field.Id] = final;
                AddReferences(final, classes, references);
                return;
            }

            var lists = new List<string[]>();
            var differs = false;
            for (var level = 1; level <= levels; level++)
            {
                var standardList = NamesOf(standard.Value(root, field.Id, level, set));
                var list = Replayed(standardList, changes);
                differs |= !list.SequenceEqual(standardList);
                lists.Add(list.ToArray());
            }

            if (!differs)
            {
                return;
            }

            for (var level = 1; level <= levels; level++)
            {
                changed[Key(field.Id, level)] = lists[level - 1];
                AddReferences(lists[level - 1], classes, references);
            }
        }

        // A list's changes done again on another list, as ObjectList does them: add at the end unless there, remove the first,
        // set the whole list, replace in the old one's place (or only take the old one out when the new one is there).
        private static List<string> Replayed(IReadOnlyList<string> start, IReadOnlyList<object[]> changes)
        {
            var list = new List<string>(start);
            foreach (var change in changes)
            {
                var names = change.Skip(1).Select(NameOf).ToList();
                switch ((string)change[0])
                {
                    case "add":
                        if (!list.Contains(names[0]))
                        {
                            list.Add(names[0]);
                        }

                        break;
                    case "remove":
                        list.Remove(names[0]);
                        break;
                    case "set":
                        list.Clear();
                        foreach (var name in names)
                        {
                            if (!list.Contains(name))
                            {
                                list.Add(name);
                            }
                        }

                        break;
                    case "replace":
                        var at = list.IndexOf(names[0]);
                        if (at >= 0 && list.Contains(names[1]))
                        {
                            list.RemoveAt(at);
                        }
                        else if (at >= 0)
                        {
                            list[at] = names[1];
                        }

                        break;
                    default:
                        throw new InvalidOperationException($"an object list holds a change the build does not know: {change[0]}");
                }
            }

            return list;
        }

        // The class names of a written list are the classes it references; an id with no class is not one.
        private static void AddReferences(IEnumerable<string> names, HashSet<string> classes, ISet<string> references)
        {
            foreach (var name in names)
            {
                if (classes.Contains(name))
                {
                    references.Add(name);
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

        // The class names of a standard list's ids, each trimmed and an empty one left out (the Hydra's abilities are
        // "Aspo, Aspt"); an id with no standard class stays as it is.
        private List<string> NamesOf(string? text)
        {
            var names = new List<string>();
            if (!string.IsNullOrEmpty(text))
            {
                foreach (var part in text.Split(','))
                {
                    var id = part.Trim();
                    if (id.Length > 0)
                    {
                        names.Add(classNames.TryGetValue(id, out var name) ? name : id);
                    }
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

        // A declared value as the build keeps it: a choice's enum value as its number, the enum's type being the map's.
        private static object? Plain(object? value)
        {
            return value is Enum choice ? Convert.ToInt32(choice, CultureInfo.InvariantCulture) : value;
        }

        // A value in the type its field holds: a float put into a whole-number field truncated, as the live setters do (the
        // kind base's Declare does it already), and a whole number put into a float field widened.
        private static object InFieldsType(StandardData.ObjectField field, object value)
        {
            if (field.Choice != null)
            {
                return value;
            }

            switch (field.Type)
            {
                case "int" when value is float real:
                    return (int)real;
                case "float" when value is int number:
                    return (float)number;
                default:
                    return value;
            }
        }

        // Whether a value is the one the standard text says; a standard object with no value reads as the type's default.
        private static bool Same(StandardData.ObjectField field, object? value, string? text)
        {
            if (field.Choice != null)
            {
                return Convert.ToInt32(value, CultureInfo.InvariantCulture) == field.Choice.Number(text);
            }

            switch (field.Type)
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
