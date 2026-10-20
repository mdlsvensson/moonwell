using System.Globalization;
using System.Text;

namespace Moonwell.Generator.Objects;

/// <summary>
/// The constructor of a standard class: it sets the values the object has in the base tables (World Editor's Default),
/// written as a map's own class sets its values, so that reading the class (Go to Definition, which shows a package's
/// classes decompiled) shows them as code.
/// </summary>
/// <remarks>
/// <para>
/// Every field the object has a value for gets one line, in the order of the property names; a field with no value
/// gets none. A value is written as the .NET side reads its text (<c>StandardValues.Parse</c> in Moonwell.Objects): an
/// <c>int</c> that is a decimal truncated, a number that is no number 0, a <c>bool</c> true for <c>1</c>, a choice as
/// its enum's members (a flags enum's joined with <c>|</c>), or a cast of the number when no member has it.
/// </para>
/// <para>
/// A field with a value for each level is <c>Name = Levels(...)</c> when its levels from 1 have values with no gap,
/// else one <c>Set("id", level, value)</c> for each level that has one. A list is <c>Name.Set&lt;...&gt;()</c> of up to
/// eight standard classes, then <c>Name.Add&lt;...&gt;()</c> for each further one; an id with no standard class of
/// the list's kind is <c>Name.Add("id")</c> in its place. A reference is <c>Name.Set&lt;...&gt;()</c>, or
/// <c>Name.Set("id")</c> for an id with no class. A list or reference with a value for each level has the one of level
/// 1, which its property reads. Both id forms are internal to Moonwell.Objects: a map names classes.
/// </para>
/// <para>
/// What a standard class sets is not declared by a map: Moonwell.Objects keeps it apart, never checks it and never
/// writes it as a change, and the build reads the map's data set for a field the map leaves unset.
/// </para>
/// </remarks>
public sealed class StandardConstructors
{
    private const string Root = "global::Moonwell.Objects";
    private const int MostInSet = 8;

    private readonly IReadOnlyDictionary<Kind, IReadOnlyList<Field>> fields;
    private readonly Dictionary<string, (ClassName Name, Kind Kind)> classes = new(StringComparer.Ordinal);

    /// <param name="fields">The fields of each kind, as the kind bases have them.</param>
    /// <param name="classes">Every standard object and its class, which the lists and references name.</param>
    public StandardConstructors(IReadOnlyDictionary<Kind, IReadOnlyList<Field>> fields, IEnumerable<(StandardObject Object, ClassName Name)> classes)
    {
        this.fields = fields;
        foreach (var (obj, name) in classes)
        {
            this.classes[obj.Id] = (name, obj.Kind);
        }
    }

    /// <summary>The statements of the object's constructor, unindented; empty for an object with no values.</summary>
    /// <param name="obj">The standard object.</param>
    /// <param name="own">An ability's own fields as its class names them; empty for any other object.</param>
    /// <exception cref="InvalidOperationException">The object has a value for a field its class has no property for.</exception>
    public IReadOnlyList<string> Lines(StandardObject obj, IReadOnlyList<Field> own)
    {
        var lines = new List<string>();
        if (obj.Values.Count == 0)
        {
            return lines;
        }

        var held = new Dictionary<string, Field>(StringComparer.Ordinal);
        foreach (var field in fields[obj.Kind])
        {
            if (field.Specific.Count == 0 && (!field.HeroOnly || obj.IsHero))
            {
                held[field.Id] = field;
            }
        }

        foreach (var field in own)
        {
            held[field.Id] = field;
        }

        var used = new Dictionary<string, Field>(StringComparer.Ordinal);
        foreach (var key in obj.Values.Keys)
        {
            var colon = key.IndexOf(':');
            var id = colon < 0 ? key : key.Substring(0, colon);
            if (!held.TryGetValue(id, out var field))
            {
                throw new InvalidOperationException($"{obj.Id} has a value for {id}, which its class has no property for");
            }

            used[id] = field;
        }

        foreach (var field in used.Values.OrderBy(f => f.Name, StringComparer.Ordinal).ThenBy(f => f.Id, StringComparer.Ordinal))
        {
            Add(lines, obj, field);
        }

        return lines;
    }

    private void Add(List<string> lines, StandardObject obj, Field field)
    {
        switch (field.Type)
        {
            case FieldType.ObjectList:
                if (FirstOf(obj, field) is { } list)
                {
                    AddList(lines, field, list);
                }

                return;
            case FieldType.ObjectRef:
                if (FirstOf(obj, field) is { } reference)
                {
                    var target = ClassOf(reference, field);
                    lines.Add(target == null ? $"{field.Name}.Set({Text(reference)});" : $"{field.Name}.Set<{target}>();");
                }

                return;
        }

        if (!field.PerLevel)
        {
            if (obj.Values.TryGetValue(field.Id, out var text))
            {
                // A flags choice of no flags is 0, which C# converts to any enum; in Levels(...) or Set the cast stays, as it gives the type.
                var value = field.Type == FieldType.Choice && field.Choice!.Flags && ChoiceNumber(field.Choice, text) == 0
                    && !field.Choice.Members.Any(member => member.Value == 0) ? "0" : Value(field, text);
                lines.Add($"{field.Name} = {value};");
            }

            return;
        }

        var levels = new SortedDictionary<int, string>();
        var prefix = field.Id + ":";
        foreach (var (key, text) in obj.Values)
        {
            if (key.StartsWith(prefix, StringComparison.Ordinal))
            {
                levels[int.Parse(key.AsSpan(prefix.Length), NumberStyles.None, CultureInfo.InvariantCulture)] = text;
            }
        }

        if (levels.Keys.SequenceEqual(Enumerable.Range(1, levels.Count)))
        {
            lines.Add($"{field.Name} = Levels({string.Join(", ", levels.Values.Select(text => Value(field, text)))});");
            return;
        }

        foreach (var (level, text) in levels)
        {
            lines.Add($"Set({Text(field.Id)}, {level.ToString(CultureInfo.InvariantCulture)}, {Value(field, text)});");
        }
    }

    // A list or reference has one value, or one for each level of which its property reads level 1's.
    private static string? FirstOf(StandardObject obj, Field field)
    {
        return obj.Values.TryGetValue(field.Id, out var text) || obj.Values.TryGetValue(field.Id + ":1", out text) ? text : null;
    }

    private void AddList(List<string> lines, Field field, string text)
    {
        var entries = text.Split(',');
        var first = entries.TakeWhile(entry => ClassOf(entry, field) != null).Take(MostInSet).Select(entry => ClassOf(entry, field)!).ToList();
        if (first.Count > 0)
        {
            lines.Add($"{field.Name}.Set<{string.Join(", ", first)}>();");
        }

        foreach (var entry in entries.Skip(first.Count))
        {
            var target = ClassOf(entry, field);
            lines.Add(target == null ? $"{field.Name}.Add({Text(entry)});" : $"{field.Name}.Add<{target}>();");
        }
    }

    /// <summary>The class an id names, spelled in full, when it is of the kind the list or reference holds; null otherwise.</summary>
    private string? ClassOf(string id, Field field)
    {
        if (!classes.TryGetValue(id, out var found))
        {
            return null;
        }

        var holds = field.ListOf ?? "object";
        return holds == "object" || holds == RuntimeOf(found.Kind) ? "global::" + found.Name.FullName : null;
    }

    // The runtime class the standard classes of a kind inherit; a hero's Hero is a Unit.
    private static string RuntimeOf(Kind kind)
    {
        return kind.ToString();
    }

    /// <summary>A value of one level, or of a field without levels, as C#.</summary>
    private static string Value(Field field, string text)
    {
        switch (field.Type)
        {
            case FieldType.Int:
                return Whole(text).ToString(CultureInfo.InvariantCulture);
            case FieldType.Float:
                return Real(text);
            case FieldType.Bool:
                return text == "1" ? "true" : "false";
            case FieldType.Choice:
                return Choice(field.Choice!, ChoiceNumber(field.Choice!, text));
            default:
                return Text(text);
        }
    }

    // As StandardValues.Parse reads an int: a whole number, else a decimal truncated, else 0.
    private static int Whole(string text)
    {
        if (int.TryParse(text, NumberStyles.Integer, CultureInfo.InvariantCulture, out var whole))
        {
            return whole;
        }

        return float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var truncated) ? (int)truncated : 0;
    }

    // As StandardValues.Parse reads a float, written so that C# reads the same float back.
    private static string Real(string text)
    {
        var value = float.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var parsed) ? parsed : 0f;
        if (float.IsNaN(value))
        {
            return "float.NaN";
        }

        if (float.IsInfinity(value))
        {
            return value > 0 ? "float.PositiveInfinity" : "float.NegativeInfinity";
        }

        return value.ToString("R", CultureInfo.InvariantCulture) + "f";
    }

    // As ChoiceTexts.Number in Moonwell.Objects reads a choice's text.
    private static int ChoiceNumber(ChoiceType choice, string text)
    {
        var trimmed = text.Trim();
        if (choice.Flags)
        {
            if (int.TryParse(trimmed, NumberStyles.Integer, CultureInfo.InvariantCulture, out var bits))
            {
                return bits;
            }

            var sum = 0;
            foreach (var part in trimmed.Split(','))
            {
                sum |= Member(choice, part.Trim())?.Value ?? 0;
            }

            return sum;
        }

        var member = Member(choice, trimmed);
        if (member != null)
        {
            return member.Value;
        }

        if (int.TryParse(trimmed, NumberStyles.Integer, CultureInfo.InvariantCulture, out var whole))
        {
            return whole;
        }

        return float.TryParse(trimmed, NumberStyles.Float, CultureInfo.InvariantCulture, out var truncated) ? (int)truncated : 0;
    }

    private static ChoiceMember? Member(ChoiceType choice, string text)
    {
        if (text.Length == 0)
        {
            return null;
        }

        foreach (var member in choice.Members)
        {
            if (string.Equals(member.Text, text, StringComparison.OrdinalIgnoreCase)
                || member.Alternates.Any(spelling => string.Equals(spelling, text, StringComparison.OrdinalIgnoreCase)))
            {
                return member;
            }
        }

        return null;
    }

    /// <summary>A choice's number as its member, a flags enum's members joined with <c>|</c>, else a cast of the number.</summary>
    private static string Choice(ChoiceType choice, int number)
    {
        var type = $"{Root}.{choice.Name}";
        var exact = choice.Members.FirstOrDefault(member => member.Value == number);
        if (exact != null)
        {
            return $"{type}.{exact.Name}";
        }

        if (choice.Flags && number != 0)
        {
            var parts = new List<string>();
            var left = number;
            foreach (var member in choice.Members)
            {
                if (member.Value != 0 && (number & member.Value) == member.Value)
                {
                    parts.Add($"{type}.{member.Name}");
                    left &= ~member.Value;
                }
            }

            if (left == 0)
            {
                return string.Join(" | ", parts);
            }
        }

        var shown = number.ToString(CultureInfo.InvariantCulture);
        return $"({type}){(number < 0 ? "(" + shown + ")" : shown)}";
    }

    /// <summary>A C# string literal; a line break or other control character is escaped.</summary>
    private static string Text(string text)
    {
        var literal = new StringBuilder("\"");
        foreach (var c in text)
        {
            switch (c)
            {
                case '"':
                    literal.Append("\\\"");
                    break;
                case '\\':
                    literal.Append("\\\\");
                    break;
                case '\n':
                    literal.Append("\\n");
                    break;
                case '\r':
                    literal.Append("\\r");
                    break;
                case '\t':
                    literal.Append("\\t");
                    break;
                default:
                    if (c < ' ' || c == '\u0085' || c == (char)0x2028 || c == (char)0x2029)
                    {
                        literal.Append("\\u").Append(((int)c).ToString("x4", CultureInfo.InvariantCulture));
                    }
                    else
                    {
                        literal.Append(c);
                    }

                    break;
            }
        }

        return literal.Append('"').ToString();
    }
}
