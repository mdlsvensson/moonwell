namespace Moonwell.Generator.Objects;

/// <summary>
/// The shared fields: a field whose live value is a member of the runtime class (<c>umpm</c> is <c>Unit.MaxMana</c>), and
/// which takes the member's name. Its kind base has no property for it: the member puts and reads the declared value while
/// the object has no handle, through the hooks the kind base overrides, and a <c>[SharedField]</c> on the kind base carries
/// its rules. The table is the 0.2.3 plan's.
/// </summary>
public static class SharedFields
{
    private static readonly Dictionary<Kind, IReadOnlyDictionary<string, string>> Members = new()
    {
        [Kind.Unit] = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            ["uhpm"] = "MaxLife",
            ["umpm"] = "MaxMana",
            ["umpi"] = "Mana",
            ["ulev"] = "Level",
            ["usca"] = "Scale",
            ["umvh"] = "FlyHeight",
            ["unam"] = "Name",
            ["ustr"] = "Strength",
            ["uagi"] = "Agility",
            ["uint"] = "Intelligence",
        },
        [Kind.Item] = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            ["iuse"] = "Charges",
        },
    };

    private static readonly HashSet<string> HeroSetById = new(StringComparer.Ordinal) { "uhpm", "umpm", "umpi", "ulev" };

    /// <summary>The shared fields of a kind, by field id to the name of the live member; empty for a kind with none.</summary>
    public static IReadOnlyDictionary<string, string> Of(Kind kind)
    {
        return Members.TryGetValue(kind, out var members) ? members : new Dictionary<string, string>();
    }

    /// <summary>The name of the live member a field shares; null for a field that is not shared.</summary>
    public static string? MemberOf(Kind kind, string id)
    {
        return Of(kind).TryGetValue(id, out var member) ? member : null;
    }

    /// <summary>
    /// Whether a standard hero's constructor sets the field by id rather than by its live name: a hero's MaxLife, MaxMana
    /// and Mana read a total, which a put by name would declare, and its level is not declared.
    /// </summary>
    public static bool HeroSetsById(string id)
    {
        return HeroSetById.Contains(id);
    }
}
