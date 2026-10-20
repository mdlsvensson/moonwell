using System.Globalization;

namespace Moonwell.Generator.Objects;

/// <summary>How the values of a choice type make one field value.</summary>
public enum ChoiceForm
{
    /// <summary>The field holds one of the values.</summary>
    One,

    /// <summary>The field holds any of the values, as a comma list of their texts; value n of the list is bit n of the enum.</summary>
    List,

    /// <summary>The field holds any of the values as a whole number: value <c>n</c> (its text) is bit n of the number.</summary>
    Bits,
}

/// <summary>One value of a choice type: one member of its enum.</summary>
/// <param name="Name">The member's C# name.</param>
/// <param name="Value">The member's number: the game's number for a value the data spells as one, its bit for flags, else its place in World Editor's list.</param>
/// <param name="Text">How the game's data and the object file spell the value: the editor's text, or the number for a value stored as a number.</param>
/// <param name="Alternates">The other spellings the game's data uses for the value; empty when none.</param>
/// <param name="Label">The editor's label for the value, without its accelerator mark (<c>&amp;</c>); empty when the export has none.</param>
public sealed record ChoiceMember(string Name, int Value, string Text, IReadOnlyList<string> Alternates, string Label);

/// <summary>A metadata type whose values World Editor offers as a list (<c>attackType</c>, <c>targetList</c>), as a C# enum.</summary>
/// <param name="Type">The metadata type, which names its section of <c>ui/uniteditordata.txt</c>.</param>
/// <param name="Name">The enum's C# name.</param>
/// <param name="Form">How the values make one field value.</param>
/// <param name="Store">How an object file stores a field of the type: <c>int</c> or <c>string</c>.</param>
/// <param name="Members">The values, in World Editor's order.</param>
public sealed record ChoiceType(string Type, string Name, ChoiceForm Form, string Store, IReadOnlyList<ChoiceMember> Members)
{
    /// <summary>True for a <c>[Flags]</c> enum: the field holds any of the values.</summary>
    public bool Flags => Form != ChoiceForm.One;
}

/// <summary>The choice types of the export: every metadata type a field uses that <c>ui/uniteditordata.txt</c> has a section for.</summary>
/// <remarks>
/// <para>
/// A member is named by its text (<c>magic</c> is <c>Magic</c>), with the word breaks of its label when the label
/// spells the same letters (<c>notself</c>, labelled <c>Not self</c>, is <c>NotSelf</c>); a text with a digit or no
/// letter (<c>AS01</c>, <c>3</c>, <c>_</c>) is named by its label alone.
/// </para>
/// <para>
/// A value the data spells as a whole number has that number (the game's numbers of <c>defenseTypeInt</c> are not in
/// World Editor's order); another value of a single choice has its place in the list. A flags enum has a bit for each
/// value: bit n for the list's value n, or for a type stored as a number (<c>morphFlags</c>), the bit whose number the
/// value's text is. The game's Lua has 32-bit integers, so a flags type of more than 31 values is an error.
/// </para>
/// </remarks>
public static class Choices
{
    private const int LabelDepth = 8;
    private const int MostBits = 31;

    // The types whose fields hold a comma list of values although their name has no List: the standard data spells a
    // unit's classification (utyp) as "undead,Mechanical" and the stat modifiers of ASde (Asdf) as "AS01,AS02,...".
    private static readonly HashSet<string> ListTypes = new(StringComparer.Ordinal) { "unitClass", "abilityModifierStats" };

    // A type stored as a number whose name ends in Flags but is one choice, not bits: fullFlags' values Never, Life
    // Only, Mana Only and Always are the numbers 0 to 3, and Rejuvenation's standard 3 is Always.
    private static readonly HashSet<string> NumberedFlags = new(StringComparer.Ordinal) { "fullFlags" };

    /// <summary>
    /// The choice types of every metadata type a field of any kind uses, by type. An enum whose name is in
    /// <paramref name="taken"/> (a type of namespace <c>Moonwell</c> or a member the standard classes inherit, either
    /// of which would hide it) gets <c>Choice</c> appended.
    /// </summary>
    public static IReadOnlyDictionary<string, ChoiceType> All(ExportTables tables, IReadOnlySet<string>? taken)
    {
        var all = new SortedDictionary<string, ChoiceType>(StringComparer.Ordinal);
        foreach (var kind in Enum.GetValues<Kind>())
        {
            foreach (var row in tables.Meta(kind))
            {
                var type = row.Columns.TryGetValue("type", out var cell) ? cell.Trim() : "";
                if (all.ContainsKey(type))
                {
                    continue;
                }

                var choice = Of(tables, type, Fields.StoreOf(type));
                if (choice != null)
                {
                    all[type] = taken != null && taken.Contains(choice.Name) ? choice with { Name = choice.Name + "Choice" } : choice;
                }
            }
        }

        return all;
    }

    /// <summary>The choice type of a metadata type stored as <paramref name="store"/>; null when <c>ui/uniteditordata.txt</c> has no section for it.</summary>
    public static ChoiceType? Of(ExportTables tables, string type, string store)
    {
        var entries = tables.Choices(type);
        return entries == null ? null : Make(type, store, entries, key => LabelOf(tables, key));
    }

    /// <summary>The choice type of a section's entries; <paramref name="label"/> gives the English label of a label key.</summary>
    /// <exception cref="InvalidOperationException">A flags type has more than 31 values, or two values get one name.</exception>
    public static ChoiceType Make(string type, string store, IReadOnlyList<ChoiceEntry> entries, Func<string, string> label)
    {
        var form = FormOf(type, store);
        var numbered = entries.All(e => IsWhole(e.Text));
        var members = new List<ChoiceMember>(entries.Count);
        var names = new HashSet<string>(StringComparer.Ordinal);
        foreach (var entry in entries)
        {
            int value;
            var text = entry.Text;
            switch (form)
            {
                case ChoiceForm.List:
                    value = Bit(type, entry.Index);
                    break;
                case ChoiceForm.Bits:
                    value = Bit(type, IsWhole(entry.Text) ? int.Parse(entry.Text, CultureInfo.InvariantCulture) : -1);
                    text = value.ToString(CultureInfo.InvariantCulture);
                    break;
                default:
                    value = numbered ? int.Parse(entry.Text, CultureInfo.InvariantCulture) : entry.Index;
                    break;
            }

            var shown = Shown(entry.Label.Length == 0 ? "" : label(entry.Label));
            var name = MemberName(entry.Text, shown);
            if (name.Length == 0)
            {
                throw new InvalidOperationException($"{type}: the value {entry.Text} has no name; it has no letter and no label");
            }

            if (!names.Add(name))
            {
                throw new InvalidOperationException($"{type}: two values are named {name}");
            }

            members.Add(new ChoiceMember(name, value, text, form == ChoiceForm.Bits ? Array.Empty<string>() : entry.Alternates, shown));
        }

        return new ChoiceType(type, EnumName(type), form, store, members);
    }

    /// <summary>
    /// The choice types with a member added for each bit that a standard value of a type stored as bits
    /// (<see cref="ChoiceForm.Bits"/>) holds and World Editor's list lacks: the game's own data uses it, so a map may
    /// write it too (<c>versionFlags</c> 6 on 20 units is The Frozen Throne and bit 4). The member is named
    /// <c>Unlisted</c> and its number (<c>Unlisted4</c>) and comes after the listed ones. Other types are returned as
    /// they are.
    /// </summary>
    /// <param name="choices">The choice types by type.</param>
    /// <param name="kinds">Each kind's fields and standard objects.</param>
    /// <exception cref="InvalidOperationException">A standard value needs bit 31 or more.</exception>
    public static IReadOnlyDictionary<string, ChoiceType> WithStandardBits(
        IReadOnlyDictionary<string, ChoiceType> choices,
        IEnumerable<(IReadOnlyList<Field> Fields, IReadOnlyList<StandardObject> Objects)> kinds)
    {
        var unlisted = new Dictionary<string, int>(StringComparer.Ordinal);
        foreach (var (fields, objects) in kinds)
        {
            var bits = fields.Where(f => f.Choice is { Form: ChoiceForm.Bits }).ToDictionary(f => f.Id, f => f.Choice!, StringComparer.Ordinal);
            foreach (var obj in objects)
            {
                foreach (var values in new[] { obj.Values }.Concat(obj.Sets.Values))
                {
                    foreach (var (key, value) in values)
                    {
                        var colon = key.IndexOf(':');
                        if (bits.TryGetValue(colon < 0 ? key : key.Substring(0, colon), out var choice)
                            && int.TryParse(value.Trim(), NumberStyles.Integer, CultureInfo.InvariantCulture, out var number))
                        {
                            var listed = choice.Members.Aggregate(0, (all, member) => all | member.Value);
                            var left = number & ~listed;
                            if (left != 0)
                            {
                                unlisted[choice.Type] = unlisted.GetValueOrDefault(choice.Type) | left;
                            }
                        }
                    }
                }
            }
        }

        var result = new SortedDictionary<string, ChoiceType>(StringComparer.Ordinal);
        foreach (var (type, choice) in choices)
        {
            if (!unlisted.TryGetValue(type, out var left))
            {
                result[type] = choice;
                continue;
            }

            var members = choice.Members.ToList();
            for (var n = 0; n < 32; n++)
            {
                if ((left & (1 << n)) != 0)
                {
                    var value = Bit(type, n);
                    var text = value.ToString(CultureInfo.InvariantCulture);
                    members.Add(new ChoiceMember("Unlisted" + text, value, text, Array.Empty<string>(), "A bit World Editor does not list, which the game's standard data uses"));
                }
            }

            result[type] = choice with { Members = members };
        }

        return result;
    }

    /// <summary>The enum's name: the type's in PascalCase, and a type ending in List names its values in the plural (<c>targetList</c> is <c>Targets</c>).</summary>
    public static string EnumName(string type)
    {
        var name = type.EndsWith("List", StringComparison.Ordinal) ? type.Substring(0, type.Length - 4) + "s" : type;
        return char.ToUpperInvariant(name[0]) + name.Substring(1);
    }

    private static ChoiceForm FormOf(string type, string store)
    {
        if (type.Contains("List", StringComparison.Ordinal) || ListTypes.Contains(type))
        {
            return ChoiceForm.List;
        }

        return store == "int" && type.EndsWith("Flags", StringComparison.Ordinal) && !NumberedFlags.Contains(type) ? ChoiceForm.Bits : ChoiceForm.One;
    }

    private static int Bit(string type, int number)
    {
        if (number < 0 || number >= MostBits)
        {
            throw new InvalidOperationException(
                $"{type}: a flags value needs bit {number}, and the game's 32-bit integers hold bits 0 to {MostBits - 1} (at most {MostBits} values)");
        }

        return 1 << number;
    }

    private static bool IsWhole(string text)
    {
        return int.TryParse(text, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out _);
    }

    private static string MemberName(string text, string label)
    {
        var fromText = FieldNames.FromLabel(text);
        var fromLabel = FieldNames.FromLabel(label);
        if (fromText.Length == 0 || fromText.TrimStart('_').Length == 0 || text.Any(char.IsAsciiDigit))
        {
            return fromLabel;
        }

        return fromLabel.Length > 0 && string.Equals(fromLabel, fromText, StringComparison.OrdinalIgnoreCase) ? fromLabel : fromText;
    }

    // A label as it reads: without the accelerator mark, and "Air, &Air" (a name and its menu entry) once.
    private static string Shown(string label)
    {
        var shown = label.Replace("&", "", StringComparison.Ordinal).Trim();
        var comma = shown.IndexOf(", ", StringComparison.Ordinal);
        if (comma > 0 && shown.Substring(0, comma) == shown.Substring(comma + 2))
        {
            return shown.Substring(0, comma);
        }

        return shown;
    }

    // A label key looked up as Fields does: a label may name another label. Empty when the export has none.
    private static string LabelOf(ExportTables tables, string key)
    {
        var label = key;
        for (var i = 0; i < LabelDepth; i++)
        {
            var next = tables.Label(label);
            if (next == label)
            {
                break;
            }

            label = next;
        }

        return label == key ? "" : label;
    }
}
