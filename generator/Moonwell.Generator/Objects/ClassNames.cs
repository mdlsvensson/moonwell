namespace Moonwell.Generator.Objects;

/// <summary>The C# namespace and class name of one standard object.</summary>
/// <param name="Namespace">The namespace (<c>Moonwell.Objects.Units.Human</c>).</param>
/// <param name="Name">The class name (<c>Paladin</c>).</param>
public sealed record ClassName(string Namespace, string Name)
{
    /// <summary>The namespace and the name, joined (<c>Moonwell.Objects.Units.Human.Paladin</c>).</summary>
    public string FullName => Namespace + "." + Name;
}

/// <summary>The class name of every standard object: the namespace by kind and race, the name from the in-game name.</summary>
public static class ClassNames
{
    private const string Root = "Moonwell.Objects";
    private const string CampaignPart = "Campaign";
    private const string OtherPart = "Other";

    private static readonly Dictionary<string, string> Races = new(StringComparer.OrdinalIgnoreCase)
    {
        ["human"] = "Human",
        ["orc"] = "Orc",
        ["undead"] = "Undead",
        ["nightelf"] = "NightElf",
        ["naga"] = "Naga",
        ["demon"] = "Demon",
        ["creeps"] = "Creeps",
        ["critters"] = "Critters",
        ["commoner"] = "Neutral",
    };

    private static readonly HashSet<string> Keywords = new(StringComparer.Ordinal)
    {
        "abstract", "as", "base", "bool", "break", "byte", "case", "catch", "char", "checked", "class", "const",
        "continue", "decimal", "default", "delegate", "do", "double", "else", "enum", "event", "explicit", "extern",
        "false", "finally", "fixed", "float", "for", "foreach", "goto", "if", "implicit", "in", "int", "interface",
        "internal", "is", "lock", "long", "namespace", "new", "null", "object", "operator", "out", "override",
        "params", "private", "protected", "public", "readonly", "ref", "return", "sbyte", "sealed", "short",
        "sizeof", "stackalloc", "static", "string", "struct", "switch", "this", "throw", "true", "try", "typeof",
        "uint", "ulong", "unchecked", "unsafe", "ushort", "using", "virtual", "void", "volatile", "while",
    };

    /// <summary>
    /// The names a class may not have: the generated kind bases, and the runtime's types, which a modder's code sees
    /// beside the standard objects.
    /// </summary>
    private static readonly HashSet<string> TakenByTypes = new(StringComparer.Ordinal)
    {
        "StandardUnit", "StandardHero", "StandardItem", "StandardDestructable", "StandardDoodad", "StandardAbility",
        "StandardBuff", "StandardUpgrade", "Unit", "Hero", "Item", "Destructable", "Doodad", "Ability", "Buff",
        "Upgrade", "Player", "Game", "Map",
    };

    /// <summary>
    /// The class name of each of <paramref name="objects"/>, keyed by kind and id. The namespace is
    /// <c>Moonwell.Objects.</c> and the kind's plural, then the race for a unit, ability, buff or upgrade
    /// (<c>Campaign</c> for a campaign unit). The name is the in-game name as <see cref="FieldNames.FromLabel"/>
    /// writes it, or the id when there is none.
    /// </summary>
    /// <remarks>
    /// A name that is a C# keyword, the last part of its namespace or a type of <see cref="TakenByTypes"/> gets the
    /// id appended. Where several objects of a namespace share a name, each gets the id appended, except the one
    /// with the first id (ordinal) when it is not a campaign object.
    /// </remarks>
    /// <exception cref="InvalidOperationException">Two objects still share a full name after that.</exception>
    public static IReadOnlyDictionary<(Kind, string), ClassName> Assign(IReadOnlyList<StandardObject> objects)
    {
        var spaces = new string[objects.Count];
        var names = new string[objects.Count];
        var groups = new Dictionary<(string, string), List<int>>();
        for (var i = 0; i < objects.Count; i++)
        {
            spaces[i] = NamespaceOf(objects[i]);
            names[i] = Plain(objects[i]);
            if (IsReserved(names[i], spaces[i]))
            {
                names[i] += Capitalize(objects[i].Id);
            }

            if (!groups.TryGetValue((spaces[i], names[i]), out var group))
            {
                groups[(spaces[i], names[i])] = group = new List<int>();
            }

            group.Add(i);
        }

        foreach (var group in groups.Values)
        {
            if (group.Count < 2)
            {
                continue;
            }

            var first = group[0];
            foreach (var i in group)
            {
                if (string.CompareOrdinal(objects[i].Id, objects[first].Id) < 0)
                {
                    first = i;
                }
            }

            foreach (var i in group)
            {
                if (i != first || objects[i].Campaign)
                {
                    names[i] += Capitalize(objects[i].Id);
                }
            }
        }

        var result = new Dictionary<(Kind, string), ClassName>(objects.Count);
        var taken = new Dictionary<string, (Kind, string)>(StringComparer.Ordinal);
        for (var i = 0; i < objects.Count; i++)
        {
            var name = new ClassName(spaces[i], names[i]);
            if (taken.TryGetValue(name.FullName, out var other))
            {
                throw new InvalidOperationException(
                    $"{other.Item1} {other.Item2} and {objects[i].Kind} {objects[i].Id} are both named {name.FullName}");
            }

            taken[name.FullName] = (objects[i].Kind, objects[i].Id);
            result[(objects[i].Kind, objects[i].Id)] = name;
        }

        return result;
    }

    private static string Plain(StandardObject o)
    {
        var name = FieldNames.FromLabel(o.Name);
        if (name.Length > 0)
        {
            return name;
        }

        return o.Id.Length > 0 && char.IsAsciiDigit(o.Id[0]) ? "_" + o.Id : o.Id;
    }

    private static bool IsReserved(string name, string space)
    {
        return Keywords.Contains(name) || TakenByTypes.Contains(name) || name == space.Substring(space.LastIndexOf('.') + 1);
    }

    private static string NamespaceOf(StandardObject o)
    {
        switch (o.Kind)
        {
            case Kind.Item:
                return Root + ".Items";
            case Kind.Destructable:
                return Root + ".Destructables";
            case Kind.Doodad:
                return Root + ".Doodads";
        }

        var kind = o.Kind switch
        {
            Kind.Unit => "Units",
            Kind.Ability => "Abilities",
            Kind.Buff => "Buffs",
            _ => "Upgrades",
        };
        if (o.Kind == Kind.Unit && o.Campaign)
        {
            return Root + "." + kind + "." + CampaignPart;
        }

        return Root + "." + kind + "." + (Races.TryGetValue(o.Race.Trim(), out var race) ? race : OtherPart);
    }

    private static string Capitalize(string text)
    {
        return text.Length == 0 ? text : char.ToUpperInvariant(text[0]) + text.Substring(1);
    }
}
