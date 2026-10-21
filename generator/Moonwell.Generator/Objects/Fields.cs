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

    /// <summary>One of the values World Editor offers, or any of them for flags: an enum of <c>Moonwell.Objects</c> (<see cref="Field.Choice"/>).</summary>
    Choice,
}

/// <summary>What World Editor allows a field to hold, from the metadata.</summary>
/// <param name="Min">The lowest value allowed, from <c>minVal</c>; null for a field with no lower limit.</param>
/// <param name="Max">The highest value allowed, from <c>maxVal</c>; null for a field with no upper limit.</param>
/// <param name="NonNegative">True when the metadata forces the value not to be negative (<c>forceNonNeg</c> 1). See <see cref="NoNegative"/> for what is checked.</param>
/// <param name="CanBeEmpty">True when the editor accepts an empty value (<c>canBeEmpty</c>).</param>
/// <param name="MaxLength">
/// For a <c>string</c> field, the most characters allowed, from a numeric <c>maxVal</c>; null when there is none or
/// when the export names a text size (<c>TTName</c>, <c>TTDesc</c>, <c>TTUber</c>), which is not a number of characters.
/// </param>
/// <param name="Label">The field's World Editor label, or null when the export has none.</param>
/// <param name="MaxEntries">
/// For a <c>stringList</c> or an object list, the most entries allowed, from a numeric <c>maxVal</c>; null when there
/// is none. The standard data shows it counts entries: upro holds 279 characters in 18 names under a maxVal of 32.
/// </param>
/// <param name="Required">
/// True for a name (a <c>string</c> with <c>canBeEmpty</c> 0 and <c>maxVal</c> <c>TTName</c>), which may not be
/// empty. Other fields with <c>canBeEmpty</c> 0 are not: standard objects leave them empty (267 units have no
/// abilities).
/// </param>
/// <param name="StandardNegative">
/// True when a standard value of the field, in the base tables or a balance set, is below 0: the game's own data holds
/// negatives there (a button at -11 is hidden, an item's bonus of -2), so a map may too. Set by
/// <see cref="Fields.WithStandardNegatives"/>.
/// </param>
public sealed record FieldRules(
    double? Min,
    double? Max,
    bool NonNegative,
    bool CanBeEmpty,
    int? MaxLength,
    string? Label,
    int? MaxEntries = null,
    bool Required = false,
    bool StandardNegative = false)
{
    /// <summary>The rules of a field with none.</summary>
    public static FieldRules None { get; } = new FieldRules(null, null, false, false, null, null);

    /// <summary>
    /// True when a number below 0 is never allowed, even past the limits: the metadata forces it
    /// (<see cref="NonNegative"/>), no standard value is negative (<see cref="StandardNegative"/>) and <see cref="Min"/>
    /// is not below 0. Otherwise a minimum of 0 is one end of the range, which <c>[BeyondLimits]</c> lifts.
    /// </summary>
    public bool NoNegative => NonNegative && !StandardNegative && !(Min < 0);
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
/// <param name="HeroOnly">
/// True for a unit field only heroes have (<c>useHero</c> without <c>useUnit</c> or <c>useBuilding</c>); false for
/// every field of another kind.
/// </param>
/// <param name="Store">How an object file stores the field's value: <c>int</c>, <c>real</c>, <c>unreal</c> or <c>string</c>.</param>
/// <param name="Rules">What World Editor allows the field to hold; <see cref="FieldRules.None"/> when not given.</param>
/// <param name="Choice">For a <see cref="FieldType.Choice"/> field, the enum it holds; null for any other type.</param>
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
    string Column,
    bool HeroOnly = false,
    string Store = "string",
    FieldRules? Rules = null,
    ChoiceType? Choice = null)
{
    /// <summary>What World Editor allows the field to hold; never null.</summary>
    public FieldRules Rules { get; init; } = Rules ?? FieldRules.None;
}

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

    /// <summary>
    /// The fields of <paramref name="kind"/>, ordered by field id. A shared field (<see cref="SharedFields"/>) is named
    /// like its live member; any other name in <paramref name="reserved"/> (the members of the runtime class the kind's
    /// objects inherit) is never given to a field: see <see cref="FieldNames.Assign"/>.
    /// A field of a type in <paramref name="choices"/> holds its enum (<see cref="Choices.All"/> of the export when
    /// null), and no property is named like an enum it does not hold: see <see cref="BesideEnums"/>.
    /// </summary>
    public static IReadOnlyList<Field> Of(
        ExportTables tables,
        Kind kind,
        IReadOnlySet<string>? reserved = null,
        IReadOnlyDictionary<string, ChoiceType>? choices = null)
    {
        choices ??= Choices.All(tables, null);
        var enums = new HashSet<string>(choices.Values.Select(c => c.Name), StringComparer.Ordinal);
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
            var choice = choices.GetValueOrDefault(Value(row, "type"));
            if (choice != null)
            {
                fieldType = FieldType.Choice;
            }

            // A shared field takes its live member's name, and keeps it as a hand-picked one.
            var member = SharedFields.MemberOf(kind, row.Id);
            var name = member ?? handPicked.GetValueOrDefault(row.Id);
            var listed = name != null;
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

            name = BesideEnums(row.Id, name!, listed, choice, repeat > 0, enums);

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
                ColumnOf(row, repeat, pointer, perField),
                kind == Kind.Unit && Value(row, "useHero") == "1" && Value(row, "useUnit") != "1" && Value(row, "useBuilding") != "1",
                StoreOf(Value(row, "type")),
                RulesOf(row, LabelOf(tables, row)),
                choice));
            candidates.Add(new NameCandidate(row.Id, name!, listed, GroupsOf(kind, row, specific, mentioned), member != null));
        }

        var names = FieldNames.Assign(candidates, reserved);
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

    /// <summary>
    /// The name a field's property gets beside the enums: inside a class, a property named like an enum hides it
    /// unless the property holds that enum itself (C# reads <c>AttackType.Magic</c> as the enum then), so a property
    /// that holds another type (<c>Levels&lt;Targets&gt; Targets</c>) gets its id appended (<c>TargetsAtar</c>).
    /// </summary>
    /// <param name="id">The field id.</param>
    /// <param name="name">The name the field asks for.</param>
    /// <param name="listed">True when the name is hand-picked.</param>
    /// <param name="holds">The enum the field holds; null for a field of another type.</param>
    /// <param name="perLevel">True when the property holds a value for each level (<c>Levels&lt;T&gt;</c>).</param>
    /// <param name="enums">The names of every enum of <c>Moonwell.Objects</c>.</param>
    /// <exception cref="InvalidOperationException">A hand-picked name is an enum's that the property does not hold.</exception>
    public static string BesideEnums(string id, string name, bool listed, ChoiceType? holds, bool perLevel, IReadOnlySet<string> enums)
    {
        if (!enums.Contains(name) || (holds != null && holds.Name == name && !perLevel))
        {
            return name;
        }

        if (listed)
        {
            throw new InvalidOperationException($"field {id} is hand-picked as {name}, the name of an enum its property does not hold");
        }

        return name + char.ToUpperInvariant(id[0]) + id.Substring(1);
    }

    /// <summary>
    /// The fields with <see cref="FieldRules.StandardNegative"/> set on each field the metadata forces not to be negative
    /// (<see cref="FieldRules.NonNegative"/>) that a standard object holds a negative in: a value under the base tables
    /// or any balance set, any level, or any number of a number list held as text. Other fields are returned as they are.
    /// </summary>
    public static IReadOnlyList<Field> WithStandardNegatives(IReadOnlyList<Field> fields, IReadOnlyList<StandardObject> objects)
    {
        var forced = new HashSet<string>(fields.Where(f => f.Rules.NonNegative).Select(f => f.Id), StringComparer.Ordinal);
        var negative = new HashSet<string>(StringComparer.Ordinal);
        foreach (var obj in objects)
        {
            foreach (var values in new[] { obj.Values }.Concat(obj.Sets.Values))
            {
                foreach (var (key, value) in values)
                {
                    var colon = key.IndexOf(':');
                    var id = colon < 0 ? key : key.Substring(0, colon);
                    if (forced.Contains(id) && !negative.Contains(id) && HasNegative(value))
                    {
                        negative.Add(id);
                    }
                }
            }
        }

        return fields.Select(f => negative.Contains(f.Id) ? f with { Rules = f.Rules with { StandardNegative = true } } : f).ToList();
    }

    private static bool HasNegative(string value)
    {
        foreach (var part in value.Split(','))
        {
            if (double.TryParse(part.Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out var number) && number < 0)
            {
                return true;
            }
        }

        return false;
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

    // The metadata types an object file stores as a whole number: the game's enums and flag sets are numbers there.
    private static readonly HashSet<string> IntTypes = new HashSet<string>(StringComparer.Ordinal)
    {
        "int", "bool", "attackBits", "channelType", "deathType", "defenseTypeInt", "detectionType", "spellDetail",
        "teamColor", "techAvail",
    };

    /// <summary>How an object file stores a value of the metadata type: <c>int</c>, <c>real</c>, <c>unreal</c> or <c>string</c>.</summary>
    internal static string StoreOf(string type)
    {
        if (IntTypes.Contains(type) || type.EndsWith("Flags", StringComparison.Ordinal))
        {
            return "int";
        }

        return type == "real" || type == "unreal" ? type : "string";
    }

    // The metadata types whose minVal and maxVal are the lowest and highest number the value may be.
    private static readonly HashSet<string> NumberTypes = new HashSet<string>(StringComparer.Ordinal)
    {
        "int", "real", "unreal", "intList", "unrealList",
    };

    private static FieldRules RulesOf(MetaRow row, string label)
    {
        var type = Value(row, "type");
        var nonNegative = Value(row, "forceNonNeg") == "1";
        double? min = null;
        double? max = null;
        int? maxLength = null;
        int? maxEntries = null;
        var canBeEmpty = Value(row, "canBeEmpty") == "1";
        var required = false;
        if (NumberTypes.Contains(type))
        {
            min = Limit(row, "minVal");
            max = Limit(row, "maxVal");
        }
        else if (type == "string")
        {
            var length = Limit(row, "maxVal", textSize: true);
            if (length.HasValue)
            {
                maxLength = (int)length.Value;
            }

            required = !canBeEmpty && Value(row, "maxVal") == "TTName";
        }
        else if (type == "stringList" || TypeOf(type).Item1 == FieldType.ObjectList)
        {
            var entries = Limit(row, "maxVal", textSize: true);
            if (entries.HasValue)
            {
                maxEntries = (int)entries.Value;
            }
        }

        return new FieldRules(min, max, nonNegative, canBeEmpty, maxLength, label.Length == 0 ? null : label, maxEntries, required);
    }

    // A limit cell as a number; null when the cell is empty, or (for a text field) names a text size such as TTDesc.
    private static double? Limit(MetaRow row, string column, bool textSize = false)
    {
        var text = Value(row, column);
        if (text.Length == 0)
        {
            return null;
        }

        if (double.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out var number))
        {
            return number;
        }

        if (textSize)
        {
            return null;
        }

        throw new FormatException($"metadata row {row.Id}: the {column} cell '{text}' is not a number");
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
