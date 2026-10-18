using System.Globalization;

namespace Moonwell.Generator.Objects;

/// <summary>The type of a field's value, from the metadata <c>type</c>.</summary>
public enum FieldType
{
    Int,
    Float,
    Bool,
    String,
    ObjectList,
    ObjectRef,
}

/// <summary>One field of one kind of object.</summary>
/// <param name="Kind">The kind of object the field belongs to.</param>
/// <param name="Id">The field id (<c>uhpm</c>), the metadata row's id.</param>
/// <param name="Name">The C# property name.</param>
/// <param name="Type">The type of the value.</param>
/// <param name="ListOf">
/// For an object list or reference, the runtime base the objects belong to: <c>Ability</c>, <c>Unit</c>,
/// <c>Item</c>, <c>Upgrade</c>, <c>Buff</c>, or <c>object</c> for a requirement list that mixes units and upgrades.
/// Null for any other type.
/// </param>
/// <param name="PerLevel">True when the field has a value for each level of the object.</param>
/// <param name="DataPointer">An ability's data field letter as a number (<c>DataA</c> is 1), else 0.</param>
/// <param name="Skin">True when the game keeps the field's value in the skin files, which every player sees.</param>
/// <param name="Specific">The ability codes the field belongs to; empty for a field of every ability.</param>
/// <param name="Column">How the field's values are keyed in <see cref="ObjectRows.Values"/>; see <see cref="Fields"/>.</param>
public sealed record Field(
    Kind Kind,
    string Id,
    string Name,
    FieldType Type,
    string? ListOf,
    bool PerLevel,
    int DataPointer,
    bool Skin,
    IReadOnlyList<string> Specific,
    string Column);

/// <summary>The fields of each kind of object, from the export's metadata tables.</summary>
/// <remarks>
/// <para>
/// A field's <see cref="Field.Column"/> is the key of its value in <see cref="ObjectRows.Values"/>:
/// </para>
/// <list type="bullet">
/// <item>An ability field that repeats for each level, held in <c>abilitydata.slk</c>, has a column per level:
/// the metadata <c>field</c>, the letter of its <c>data</c> pointer (A for 1), and <c>{level}</c> where the level
/// number goes (<c>DataA{level}</c>, <c>Cool{level}</c>).</item>
/// <item>A field that repeats and is held in the text files (a tooltip) has one column with a value for each level
/// in a comma list, level 1 first, and no <c>{level}</c>. The exception is an upgrade's requirement list, whose
/// level 1 is the bare column (<c>Requires</c>) and whose level n is the column with n - 1 appended
/// (<c>Requires1</c>).</item>
/// <item>A field that shares its metadata <c>field</c> with other fields of the kind is one element of a comma
/// list, and its column ends with the element's position (<c>Buttonpos[0]</c> is the X of <c>Buttonpos=2,3</c>).</item>
/// <item>Any other field's column is its metadata <c>field</c>.</item>
/// </list>
/// <para>
/// Units and items take their fields from the same metadata table; the use flags tell which kind has which. Every
/// field of the table is a field of the other kinds.
/// </para>
/// </remarks>
public static class Fields
{
    private const string Text = "Profile";
    private const int LabelDepth = 8;

    /// <summary>The fields of <paramref name="kind"/>, ordered by field id.</summary>
    public static IReadOnlyList<Field> Of(ExportTables tables, Kind kind)
    {
        var rows = new List<MetaRow>();
        foreach (var row in tables.Meta(kind))
        {
            if (Applies(kind, row))
            {
                rows.Add(row);
            }
        }

        rows.Sort((a, b) => string.CompareOrdinal(a.Id, b.Id));
        var perField = new Dictionary<string, int>(StringComparer.Ordinal);
        foreach (var row in rows)
        {
            var field = Value(row, "field");
            perField[field] = perField.GetValueOrDefault(field) + 1;
        }

        var mentioned = kind == Kind.Ability ? MentionedAbilities(rows) : new List<string>();
        var handPicked = FieldNames.HandPicked(kind);
        var fields = new List<Field>();
        var candidates = new List<NameCandidate>();
        foreach (var row in rows)
        {
            var repeat = Number(row, "repeat");
            var pointer = Number(row, "data");
            var specific = Split(Value(row, "useSpecific"));
            var (fieldType, listOf) = TypeOf(Value(row, "type"));
            var listed = handPicked.TryGetValue(row.Id, out var name);
            if (!listed)
            {
                name = FieldNames.FromLabel(LabelOf(tables, row));
                if (name!.Length == 0)
                {
                    name = FieldNames.FromLabel(Value(row, "field"));
                }

                if (name.Length == 0)
                {
                    name = FieldNames.FromLabel(row.Id);
                }
            }

            fields.Add(new Field(
                kind,
                row.Id,
                name!,
                fieldType,
                listOf,
                repeat > 0,
                pointer,
                Value(row, "netsafe") == "1",
                specific,
                ColumnOf(row, repeat, pointer, perField)));
            candidates.Add(new NameCandidate(row.Id, name!, listed, GroupsOf(kind, row, specific, mentioned)));
        }

        var names = FieldNames.Assign(candidates);
        var result = new List<Field>(fields.Count);
        for (var i = 0; i < fields.Count; i++)
        {
            result.Add(fields[i] with { Name = names[i] });
        }

        return result;
    }

    private static bool Applies(Kind kind, MetaRow row)
    {
        switch (kind)
        {
            case Kind.Unit:
                return Value(row, "useUnit") == "1" || Value(row, "useHero") == "1" || Value(row, "useBuilding") == "1";
            case Kind.Item:
                return Value(row, "useItem") == "1";
            default:
                return true;
        }
    }

    private static string Value(MetaRow row, string column)
    {
        return row.Columns.TryGetValue(column, out var value) ? value.Trim() : "";
    }

    private static int Number(MetaRow row, string column)
    {
        var text = Value(row, column);
        if (text.Length == 0)
        {
            return 0;
        }

        if (!int.TryParse(text, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out var number))
        {
            throw new FormatException($"metadata row {row.Id}: the {column} cell '{text}' is not a whole number");
        }

        return number;
    }

    private static (FieldType, string?) TypeOf(string type)
    {
        switch (type)
        {
            case "int":
                return (FieldType.Int, null);
            case "real":
            case "unreal":
                return (FieldType.Float, null);
            case "bool":
                return (FieldType.Bool, null);
            case "abilityList":
            case "heroAbilityList":
                return (FieldType.ObjectList, "Ability");
            case "unitList":
                return (FieldType.ObjectList, "Unit");
            case "itemList":
                return (FieldType.ObjectList, "Item");
            case "upgradeList":
                return (FieldType.ObjectList, "Upgrade");
            case "buffList":
                return (FieldType.ObjectList, "Buff");
            case "techList":
                return (FieldType.ObjectList, "object");
            case "abilCode":
                return (FieldType.ObjectRef, "Ability");
            case "unitCode":
                return (FieldType.ObjectRef, "Unit");
            case "upgradeCode":
                return (FieldType.ObjectRef, "Upgrade");
            case "itemCode":
                return (FieldType.ObjectRef, "Item");
            case "buffCode":
                return (FieldType.ObjectRef, "Buff");
            default:
                return (FieldType.String, null);
        }
    }

    private static string ColumnOf(MetaRow row, int repeat, int pointer, Dictionary<string, int> perField)
    {
        var field = Value(row, "field");
        var column = pointer > 0 ? field + (char)('A' + pointer - 1) : field;
        if (repeat > 0 && Value(row, "slk") != Text)
        {
            return column + "{level}";
        }

        var index = Number(row, "index");
        if (perField[field] > 1 && Value(row, "index").Length > 0 && index >= 0)
        {
            return column + "[" + index.ToString(CultureInfo.InvariantCulture) + "]";
        }

        return column;
    }

    /// <summary>
    /// The editor's label of a field: <c>displayName</c> looked up (it may name another label) and, for an upgrade
    /// effect, <c>%s</c> replaced by the row's <c>effectType</c>. The empty string when the export has no label.
    /// </summary>
    private static string LabelOf(ExportTables tables, MetaRow row)
    {
        var key = Value(row, "displayName");
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

        if (label == key)
        {
            return "";
        }

        if (label.Contains("%s", StringComparison.Ordinal))
        {
            label = label.Replace("%s", Value(row, "effectType"), StringComparison.Ordinal).TrimEnd();
            if (label.EndsWith('-'))
            {
                label = label.Substring(0, label.Length - 1).TrimEnd();
            }
        }

        return label;
    }

    private static List<string> Split(string cell)
    {
        var codes = new List<string>();
        foreach (var part in cell.Split(',', '.'))
        {
            var code = part.Trim();
            if (code.Length > 0)
            {
                codes.Add(code);
            }
        }

        return codes;
    }

    /// <summary>Every ability code a row names in <c>useSpecific</c> or <c>notSpecific</c>.</summary>
    private static List<string> MentionedAbilities(List<MetaRow> rows)
    {
        var seen = new HashSet<string>(StringComparer.Ordinal);
        var codes = new List<string>();
        foreach (var row in rows)
        {
            foreach (var code in Split(Value(row, "useSpecific") + "," + Value(row, "notSpecific")))
            {
                if (seen.Add(code))
                {
                    codes.Add(code);
                }
            }
        }

        return codes;
    }

    /// <summary>
    /// The classes a field is a property of, for telling which fields can meet in one class: a unit field is in the
    /// unit, hero and building classes it is used by; an ability field is in the classes of its specific abilities,
    /// or in every ability's class; any other field is in its kind's one class.
    /// </summary>
    private static List<string> GroupsOf(Kind kind, MetaRow row, List<string> specific, List<string> mentioned)
    {
        var groups = new List<string>();
        switch (kind)
        {
            case Kind.Unit:
                foreach (var use in new[] { "useUnit", "useHero", "useBuilding" })
                {
                    if (Value(row, use) == "1")
                    {
                        groups.Add(use);
                    }
                }

                break;
            case Kind.Ability when specific.Count > 0:
                groups.AddRange(specific);
                break;
            case Kind.Ability:
                var not = Split(Value(row, "notSpecific"));
                groups.Add("common");
                foreach (var code in mentioned)
                {
                    if (!not.Contains(code))
                    {
                        groups.Add(code);
                    }
                }

                break;
            default:
                groups.Add("all");
                break;
        }

        return groups;
    }
}
